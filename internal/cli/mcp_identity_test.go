package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/auth"
)

const testIdentityIssuer = "https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0"

// identityProbe は identity ミドルウェア通過後に、ハンドラーから見える状態を記録する。
type identityProbe struct {
	called      bool
	userID      string
	userIDOK    bool
	seenIssuer  []string
	seenSubject []string
}

func newIdentityProbe() (*identityProbe, http.Handler) {
	p := &identityProbe{}
	return p, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.called = true
		p.userID, p.userIDOK = auth.UserIDFromContext(r.Context())
		p.seenIssuer = r.Header.Values(identityIssuerHeaderName)
		p.seenSubject = r.Header.Values(identitySubjectHeaderName)
		w.WriteHeader(http.StatusOK)
	})
}

// serveIdentity は identity ミドルウェアだけを通したリクエストを実行する。
func serveIdentity(t *testing.T, setup func(*http.Request)) (*httptest.ResponseRecorder, *identityProbe) {
	t.Helper()
	probe, next := newIdentityProbe()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	if setup != nil {
		setup(req)
	}
	w := httptest.NewRecorder()
	identityMiddleware()(next).ServeHTTP(w, req)
	return w, probe
}

func setIdentity(issuer, subject string) func(*http.Request) {
	return func(r *http.Request) {
		r.Header.Set(identityIssuerHeaderName, issuer)
		r.Header.Set(identitySubjectHeaderName, subject)
	}
}

// 契約 §2.2: iss + sub の組を安定した userID へ写す。
func TestIdentityMiddleware_InjectsUserID(t *testing.T) {
	w, probe := serveIdentity(t, setIdentity(testIdentityIssuer, "abc123-sub"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !probe.called {
		t.Fatal("next handler was not called")
	}
	if !probe.userIDOK {
		t.Fatal("UserIDFromContext returned ok=false, want a derived userID")
	}
	want := auth.NormalizeUserID(testIdentityIssuer, "abc123-sub")
	if probe.userID != want {
		t.Errorf("userID = %q, want %q", probe.userID, want)
	}
}

// userID は iss+sub の組に対して安定し、かつ組が違えば衝突しない。
func TestNormalizeUserID_StableAndDistinct(t *testing.T) {
	a := auth.NormalizeUserID(testIdentityIssuer, "sub-1")
	if a != auth.NormalizeUserID(testIdentityIssuer, "sub-1") {
		t.Error("NormalizeUserID is not stable for the same iss+sub")
	}
	if a == auth.NormalizeUserID(testIdentityIssuer, "sub-2") {
		t.Error("different subjects mapped to the same userID")
	}
	if a == auth.NormalizeUserID("https://other.example/v2.0", "sub-1") {
		t.Error("different issuers mapped to the same userID")
	}

	// 区切りに NUL を使うため、iss/sub の境界をずらした組は衝突しない。
	if auth.NormalizeUserID("https://ex/a", "b") == auth.NormalizeUserID("https://ex/", "ab") {
		t.Error("boundary shift produced a colliding userID")
	}

	sum := sha256.Sum256([]byte(testIdentityIssuer + "\x00" + "sub-1"))
	if a != hex.EncodeToString(sum[:]) {
		t.Errorf("userID = %q, want sha256 hex of iss\\x00sub", a)
	}
}

// 契約 §2.4: 受信した identity ヘッダーは strip し、ハンドラーには検証済み値のみを渡す。
func TestIdentityMiddleware_StripsHeadersFromDownstream(t *testing.T) {
	_, probe := serveIdentity(t, setIdentity(testIdentityIssuer, "abc123-sub"))

	if len(probe.seenIssuer) != 0 || len(probe.seenSubject) != 0 {
		t.Errorf("downstream saw identity headers issuer=%v subject=%v, want none", probe.seenIssuer, probe.seenSubject)
	}
}

// strip は元のリクエストを破壊しない（クローンに対して行う）。
func TestIdentityMiddleware_DoesNotMutateOriginalRequest(t *testing.T) {
	probe, next := newIdentityProbe()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	setIdentity(testIdentityIssuer, "abc123-sub")(req)

	identityMiddleware()(next).ServeHTTP(httptest.NewRecorder(), req)

	if !probe.called {
		t.Fatal("next handler was not called")
	}
	if req.Header.Get(identityIssuerHeaderName) != testIdentityIssuer {
		t.Error("original request header was mutated by the middleware")
	}
}

// 同名ヘッダーが複数届いた場合、どれが Gateway 由来か区別できないため拒否する
// （クライアント由来の値が append されている可能性を排除できない）。
func TestIdentityMiddleware_RejectsDuplicateHeaders(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*http.Request)
	}{
		{"duplicate issuer", func(r *http.Request) {
			r.Header.Add(identityIssuerHeaderName, "https://attacker.example/v2.0")
			r.Header.Add(identityIssuerHeaderName, testIdentityIssuer)
			r.Header.Set(identitySubjectHeaderName, "abc123-sub")
		}},
		{"duplicate subject", func(r *http.Request) {
			r.Header.Set(identityIssuerHeaderName, testIdentityIssuer)
			r.Header.Add(identitySubjectHeaderName, "victim-sub")
			r.Header.Add(identitySubjectHeaderName, "attacker-sub")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, probe := serveIdentity(t, tt.setup)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if probe.called {
				t.Error("next handler was called for an ambiguous identity")
			}
			assertErrorEnvelope(t, w, "invalid_identity")
		})
	}
}

// 契約 §2.3 + 決定D: identity 欠落時に単一テナント固定 userID へフォールバックしない。
func TestIdentityMiddleware_MissingHeaders(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*http.Request)
	}{
		{"both missing", nil},
		{"issuer only", func(r *http.Request) {
			r.Header.Set(identityIssuerHeaderName, testIdentityIssuer)
		}},
		{"subject only", func(r *http.Request) {
			r.Header.Set(identitySubjectHeaderName, "abc123-sub")
		}},
		{"empty issuer", setIdentity("", "abc123-sub")},
		{"empty subject", setIdentity(testIdentityIssuer, "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, probe := serveIdentity(t, tt.setup)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", w.Code)
			}
			if probe.called {
				t.Error("next handler was called without an established identity")
			}
			assertErrorEnvelope(t, w, "identity_required")
		})
	}
}

// userID の形式検証（長さ・文字種）。
func TestIdentityMiddleware_RejectsMalformedValues(t *testing.T) {
	long := "https://ex.example/" + strings.Repeat("a", maxIdentityValueLen)
	tests := []struct {
		name    string
		issuer  string
		subject string
	}{
		{"issuer too long", long, "abc123-sub"},
		{"subject too long", testIdentityIssuer, strings.Repeat("s", maxIdentityValueLen+1)},
		{"subject with space", testIdentityIssuer, "abc 123"},
		{"subject with tab", testIdentityIssuer, "abc\t123"},
		{"subject with comma", testIdentityIssuer, "abc,123"},
		{"issuer with comma", testIdentityIssuer + ",https://attacker.example/v2.0", "abc123-sub"},
		{"non-ascii subject", testIdentityIssuer, "サブジェクト"},
		{"del control char", testIdentityIssuer, "abc\x7f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, probe := serveIdentity(t, setIdentity(tt.issuer, tt.subject))
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if probe.called {
				t.Error("next handler was called for a malformed identity")
			}
			assertErrorEnvelope(t, w, "invalid_identity")
		})
	}
}

func TestIdentityMiddleware_AcceptsBoundaryLengths(t *testing.T) {
	subject := strings.Repeat("s", maxIdentityValueLen)
	w, probe := serveIdentity(t, setIdentity(testIdentityIssuer, subject))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if probe.userID != auth.NormalizeUserID(testIdentityIssuer, subject) {
		t.Error("boundary-length subject was not accepted as-is")
	}
}

// 契約 §2.3: apikey 検証を通過しないリクエストの identity ヘッダーは一切参照しない。
// identity ミドルウェアは apikey ミドルウェアの内側に配線されるため、apikey 不成立の
// リクエストは identity ミドルウェアに到達しない。
func TestIdentityMiddleware_NotReachedWithoutAPIKey(t *testing.T) {
	key := strings.Repeat("k", 32)
	probe, next := newIdentityProbe()
	handler := apiKeyAuthMiddleware(key)(identityMiddleware()(next))

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	setIdentity(testIdentityIssuer, "abc123-sub")(req)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if probe.called {
		t.Error("next handler was called without a valid api key")
	}
	assertErrorEnvelope(t, w, "unauthorized")
}

func TestIdentityMiddleware_TrustedAfterAPIKey(t *testing.T) {
	key := strings.Repeat("k", 32)
	probe, next := newIdentityProbe()
	handler := apiKeyAuthMiddleware(key)(identityMiddleware()(next))

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	req.Header.Set(apiKeyHeaderName, key)
	setIdentity(testIdentityIssuer, "abc123-sub")(req)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !probe.userIDOK {
		t.Error("userID was not injected for an api-key-authenticated request")
	}
}

// auth-mode=none では identity を注入せず、クライアント由来のヘッダーは strip する
// （契約 §2.3: apikey 検証を通過していない identity は信用しない）。
func TestStripIdentityMiddleware_NoInjection(t *testing.T) {
	probe, next := newIdentityProbe()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	setIdentity(testIdentityIssuer, "abc123-sub")(req)
	w := httptest.NewRecorder()

	stripIdentityMiddleware()(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if probe.userIDOK {
		t.Errorf("userID = %q was injected in auth-mode=none, want none", probe.userID)
	}
	if len(probe.seenIssuer) != 0 || len(probe.seenSubject) != 0 {
		t.Errorf("downstream saw identity headers issuer=%v subject=%v, want none", probe.seenIssuer, probe.seenSubject)
	}
}

func assertErrorEnvelope(t *testing.T, w *httptest.ResponseRecorder, wantCode string) {
	t.Helper()

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var env struct {
		SchemaVersion string `json:"schema_version"`
		Error         struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not a JSON error envelope: %v (body=%s)", err, w.Body.String())
	}
	if env.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want \"1\"", env.SchemaVersion)
	}
	if env.Error.Code != wantCode {
		t.Errorf("error.code = %q, want %q", env.Error.Code, wantCode)
	}
	if env.Error.Message == "" {
		t.Error("error.message is empty")
	}
	if env.Error.Retryable {
		t.Error("error.retryable = true, want false")
	}
}
