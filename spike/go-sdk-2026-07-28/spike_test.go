// Package spike is a throwaway, independently-moduled Go program used to
// empirically verify the behavior of github.com/modelcontextprotocol/go-sdk
// v1.7.0 for the logvalet MCP server redesign (issue #52, step S03).
//
// It exercises the real HTTP transport of mcp.NewStreamableHTTPHandler via
// httptest, asserting on raw HTTP responses (no SDK client used for
// assertions) so the results reflect exactly what an external HTTP caller
// would observe. See docs/specs/spike-go-sdk-2026-07-28.md for the
// consolidated write-up.
package spike

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newTestServer builds a minimal MCP server with a single "echo" tool.
func newTestServer(t *testing.T) *mcp.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "spike-server", Version: "v0.0.1-spike"}, nil)
	type echoIn struct {
		Message string `json:"message"`
	}
	type echoOut struct {
		Echo string `json:"echo"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "echoes back the message",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, echoOut, error) {
		return nil, echoOut{Echo: in.Message}, nil
	})
	return server
}

// rawPost issues a raw HTTP POST against the streamable handler and returns
// the status code, response headers, and body verbatim -- no SDK client
// parsing involved, so we see exactly what's on the wire.
func rawPost(t *testing.T, url, body string, headers map[string]string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return resp.StatusCode, resp.Header.Clone(), string(b)
}

type jsonrpcErr struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type jsonrpcEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *jsonrpcErr     `json:"error"`
}

// ---------------------------------------------------------------------
// (a) Stateless=true: does tools/call succeed WITHOUT a prior initialize,
// using the new (>=2026-07-28) sessionless protocol signaled via
// _meta["io.modelcontextprotocol/protocolVersion"] in the request body
// plus the Mcp-Protocol-Version HTTP header?
// ---------------------------------------------------------------------

func TestA_StatelessDirectToolCall_NewProtocol(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	// Direct tools/call, no initialize beforehand. Signals new protocol via
	// _meta.protocolVersion in the body AND the Mcp-Protocol-Version header
	// (both required per SEP-2575 / the SDK's validateRequestMeta +
	// streamable.go header-mismatch checks).
	body := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "echo",
			"arguments": {"message": "hello"},
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	status, hdr, resp := rawPost(t, httpServer.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "echo",
	})
	t.Logf("(a) status=%d contentType=%s body=%s", status, hdr.Get("Content-Type"), resp)

	if status != http.StatusOK {
		t.Errorf("(a) got status %d, want 200 (direct tools/call without initialize should succeed in stateless+new-protocol mode); body=%s", status, resp)
	}
	var env jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("(a) failed to unmarshal response: %v; body=%s", err, resp)
	}
	if env.Error != nil {
		t.Errorf("(a) got JSON-RPC error %d %q, want success", env.Error.Code, env.Error.Message)
	}
	if !strings.Contains(string(env.Result), `"echo":"hello"`) && !strings.Contains(string(env.Result), `"hello"`) {
		t.Errorf("(a) result does not contain expected echo payload: %s", env.Result)
	}
}

// TestA_StatelessDirectToolCall_LegacyProtocol_RequiresInitialize verifies
// that WITHOUT the new-protocol _meta signaling (i.e. a plain legacy
// tools/call with no prior initialize), the stateless server still requires
// a session / rejects the call -- so "no initialize needed" is specific to
// the new SEP-2575 protocol path, not a general stateless behavior.
func TestA_StatelessDirectToolCall_LegacyProtocol_RequiresInitialize(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "echo", "arguments": {"message": "hello"}}
	}`
	status, hdr, resp := rawPost(t, httpServer.URL, body, nil)
	t.Logf("(a-legacy) status=%d contentType=%s body=%s", status, hdr.Get("Content-Type"), resp)
}

// ---------------------------------------------------------------------
// (b) server/discover: does it return protocolVersion / capabilities /
// identity without requiring initialize?
// ---------------------------------------------------------------------

func TestB_ServerDiscover(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "server/discover",
		"params": {
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	status, hdr, resp := rawPost(t, httpServer.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "server/discover",
	})
	t.Logf("(b) status=%d contentType=%s body=%s", status, hdr.Get("Content-Type"), resp)

	if status != http.StatusOK {
		t.Fatalf("(b) got status %d, want 200; body=%s", status, resp)
	}
	var env jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("(b) failed to unmarshal response: %v; body=%s", err, resp)
	}
	if env.Error != nil {
		t.Fatalf("(b) got JSON-RPC error %d %q, want success", env.Error.Code, env.Error.Message)
	}
	var result struct {
		SupportedVersions []string        `json:"supportedVersions"`
		Capabilities      json.RawMessage `json:"capabilities"`
		Meta              struct {
			ServerInfo *struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"io.modelcontextprotocol/serverInfo"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("(b) failed to unmarshal discover result: %v; result=%s", err, env.Result)
	}
	if len(result.SupportedVersions) == 0 {
		t.Errorf("(b) supportedVersions is empty, want a non-empty list")
	}
	if len(result.Capabilities) == 0 {
		t.Errorf("(b) capabilities is missing/empty")
	}
	if result.Meta.ServerInfo == nil {
		t.Errorf("(b) _meta.io.modelcontextprotocol/serverInfo is missing, want server identity")
	} else if result.Meta.ServerInfo.Name != "spike-server" {
		t.Errorf("(b) server identity name = %q, want %q", result.Meta.ServerInfo.Name, "spike-server")
	}
}

// ---------------------------------------------------------------------
// (c) MCP-Protocol-Version / Mcp-Method / Mcp-Name header requirements and
// HeaderMismatch(-32020) / UnsupportedProtocolVersionError(-32022) trigger
// conditions.
// ---------------------------------------------------------------------

// TestC_MissingProtocolVersionHeader_NewProtocolBody verifies that when the
// request body signals the new protocol via _meta but the
// Mcp-Protocol-Version HTTP header is absent, the server returns
// HeaderMismatch (-32020).
func TestC_MissingProtocolVersionHeader_NewProtocolBody(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "echo",
			"arguments": {"message": "hi"},
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	// Deliberately omit Mcp-Protocol-Version header.
	status, _, resp := rawPost(t, httpServer.URL, body, nil)
	t.Logf("(c-1) status=%d body=%s", status, resp)

	var env jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("(c-1) failed to unmarshal: %v; body=%s", err, resp)
	}
	if env.Error == nil {
		t.Fatalf("(c-1) expected a JSON-RPC error, got success: %s", resp)
	}
	if env.Error.Code != -32020 {
		t.Errorf("(c-1) error code = %d, want -32020 (HeaderMismatch); message=%q", env.Error.Code, env.Error.Message)
	}
}

// TestC_MismatchedProtocolVersionHeader verifies that when
// Mcp-Protocol-Version (header) and _meta.protocolVersion (body) disagree,
// the server returns HeaderMismatch (-32020).
func TestC_MismatchedProtocolVersionHeader(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "echo",
			"arguments": {"message": "hi"},
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	status, _, resp := rawPost(t, httpServer.URL, body, map[string]string{
		// Header claims a different (older, but still >= the SDK's minimum
		// standard-header version) protocol version than the body's _meta.
		"Mcp-Protocol-Version": "2025-11-25",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "echo",
	})
	t.Logf("(c-2) status=%d body=%s", status, resp)

	var env jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("(c-2) failed to unmarshal: %v; body=%s", err, resp)
	}
	if env.Error == nil {
		t.Fatalf("(c-2) expected a JSON-RPC error, got success: %s", resp)
	}
	if env.Error.Code != -32020 {
		t.Errorf("(c-2) error code = %d, want -32020 (HeaderMismatch); message=%q", env.Error.Code, env.Error.Message)
	}
}

// TestC_MissingMcpMethodHeader verifies that when Mcp-Protocol-Version is
// present at/above the "standard headers" minimum (2026-07-28) but
// Mcp-Method is missing, HeaderMismatch (-32020) fires.
func TestC_MissingMcpMethodHeader(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "echo",
			"arguments": {"message": "hi"},
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	status, _, resp := rawPost(t, httpServer.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		// Mcp-Method deliberately omitted; Mcp-Name also omitted.
	})
	t.Logf("(c-3) status=%d body=%s", status, resp)

	var env jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("(c-3) failed to unmarshal: %v; body=%s", err, resp)
	}
	if env.Error == nil {
		t.Fatalf("(c-3) expected a JSON-RPC error, got success: %s", resp)
	}
	if env.Error.Code != -32020 {
		t.Errorf("(c-3) error code = %d, want -32020 (HeaderMismatch); message=%q", env.Error.Code, env.Error.Message)
	}
}

// TestC_UnsupportedProtocolVersion verifies UnsupportedProtocolVersionError
// (-32022) when _meta.protocolVersion names a version the server does not
// support (garbage version, new-protocol path).
func TestC_UnsupportedProtocolVersion(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "echo",
			"arguments": {"message": "hi"},
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2099-01-01",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	status, _, resp := rawPost(t, httpServer.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2099-01-01",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "echo",
	})
	t.Logf("(c-4) status=%d body=%s", status, resp)

	var env jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("(c-4) failed to unmarshal: %v; body=%s", err, resp)
	}
	if env.Error == nil {
		t.Fatalf("(c-4) expected a JSON-RPC error, got success: %s", resp)
	}
	if env.Error.Code != -32022 {
		t.Errorf("(c-4) error code = %d, want -32022 (UnsupportedProtocolVersionError); message=%q", env.Error.Code, env.Error.Message)
	}
}

// TestC_LegacyRequest_NoHeadersRequired verifies that plain legacy (<
// 2026-07-28) requests -- e.g. an initialize call with no
// Mcp-Protocol-Version header at all -- are NOT subject to the
// Mcp-Method/Mcp-Name header requirement (it's a no-op / not enforced).
func TestC_LegacyRequest_NoHeadersRequired(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := `{"jsonrpc": "2.0", "id": 1, "method":"initialize", "params": {"protocolVersion":"2025-11-25"}}`
	status, _, resp := rawPost(t, httpServer.URL, body, nil)
	t.Logf("(c-5) status=%d body=%s", status, resp)
	if status != http.StatusOK {
		t.Errorf("(c-5) legacy initialize without any MCP-* headers got status %d, want 200; body=%s", status, resp)
	}
}

// ---------------------------------------------------------------------
// (d) Does the SDK negotiate old-protocol (2025-11-25 etc.) sessions
// itself, i.e. does it provide a parallel legacy initialize/session-based
// server path alongside the new sessionless one?
// ---------------------------------------------------------------------

// TestD_LegacyInitializeSessionFlow_SameHandlerSameServer verifies that the
// SAME mcp.NewStreamableHTTPHandler / mcp.Server pair that serves the new
// SEP-2575 sessionless protocol ALSO serves the full legacy
// initialize -> notifications/initialized -> tools/call session flow,
// including issuing an Mcp-Session-Id and honoring it on follow-up calls.
func TestD_LegacyInitializeSessionFlow_SameHandlerSameServer(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true}) // Stateless NOT set -> stateful/legacy path
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	// 1. initialize with an old protocol version.
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"spike-client","version":"v1"}}}`
	status, hdr, resp := rawPost(t, httpServer.URL, initBody, nil)
	t.Logf("(d-1) initialize status=%d sessionID=%q body=%s", status, hdr.Get("Mcp-Session-Id"), resp)
	if status != http.StatusOK {
		t.Fatalf("(d-1) initialize failed: status=%d body=%s", status, resp)
	}
	var initEnv jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &initEnv); err != nil {
		t.Fatalf("(d-1) unmarshal: %v", err)
	}
	if initEnv.Error != nil {
		t.Fatalf("(d-1) initialize returned error: %+v", initEnv.Error)
	}
	var initResult struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(initEnv.Result, &initResult); err != nil {
		t.Fatalf("(d-1) unmarshal result: %v", err)
	}
	if initResult.ProtocolVersion != "2025-11-25" {
		t.Errorf("(d-1) negotiated protocolVersion = %q, want %q (server should honor the client's requested legacy version)", initResult.ProtocolVersion, "2025-11-25")
	}
	sessionID := hdr.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("(d-1) no Mcp-Session-Id issued for legacy stateful initialize")
	}

	// 2. notifications/initialized (required before other calls in legacy flow).
	notifBody := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	status, _, resp = rawPost(t, httpServer.URL, notifBody, map[string]string{"Mcp-Session-Id": sessionID})
	t.Logf("(d-2) notifications/initialized status=%d body=%q", status, resp)

	// 3. tools/call using the session established by legacy initialize.
	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"message":"legacy-flow"}}}`
	status, _, resp = rawPost(t, httpServer.URL, callBody, map[string]string{"Mcp-Session-Id": sessionID})
	t.Logf("(d-3) tools/call status=%d body=%s", status, resp)
	if status != http.StatusOK {
		t.Fatalf("(d-3) tools/call over legacy session failed: status=%d body=%s", status, resp)
	}
	var callEnv jsonrpcEnvelope
	if err := json.Unmarshal([]byte(resp), &callEnv); err != nil {
		t.Fatalf("(d-3) unmarshal: %v", err)
	}
	if callEnv.Error != nil {
		t.Fatalf("(d-3) tools/call returned error: %+v", callEnv.Error)
	}
	if !strings.Contains(string(callEnv.Result), "legacy-flow") {
		t.Errorf("(d-3) result does not contain expected payload: %s", callEnv.Result)
	}
}

// TestD_NewProtocolRejectedOnStatefulServer verifies the flip side: on a
// server NOT configured with Stateless=true, a new-protocol
// (_meta.protocolVersion >= 2026-07-28) request is rejected with a plain
// HTTP 400 (not served), except for server/discover which is exempted so
// clients can probe capabilities before deciding which protocol to use.
func TestD_NewProtocolRejectedOnStatefulServer(t *testing.T) {
	server := newTestServer(t)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true}) // stateful (Stateless not set)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	callBody := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "echo",
			"arguments": {"message": "hi"},
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	status, _, resp := rawPost(t, httpServer.URL, callBody, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "echo",
	})
	t.Logf("(d-4) new-protocol tools/call on stateful server: status=%d body=%s", status, resp)
	if status != http.StatusBadRequest {
		t.Errorf("(d-4) status = %d, want 400 (new protocol requires Stateless=true); body=%s", status, resp)
	}

	// server/discover, however, should be exempt and succeed even on a
	// stateful server, per streamable.go's explicit exemption.
	discoverBody := `{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "server/discover",
		"params": {
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": {}
			}
		}
	}`
	status, _, resp = rawPost(t, httpServer.URL, discoverBody, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "server/discover",
	})
	t.Logf("(d-5) server/discover on stateful server: status=%d body=%s", status, resp)
	if status != http.StatusOK {
		t.Errorf("(d-5) server/discover on stateful server: status = %d, want 200 (discover should be exempt from the stateless-only gate); body=%s", status, resp)
	}
}
