package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// snapshot_test.go は S06 done_criteria(2) の実装。
//
// testdata/tools_list_baseline.json は移行前 (mark3labs/mcp-go backend) の
// "tools/list" JSON-RPC レスポンス全体を、下記の正規化規則を適用した正規形で固定した
// golden ファイル。S07/S09 で ToolDef ベースの新実装に切り替えた後の tools/list
// レスポンスも同じ normalizeToolsListResponse を通した上でこの baseline と比較することで、
// 移行前後で観測可能なプロトコル応答が変わっていないことを検証する唯一の基準とする。
//
// 取得プロトコル版: MCP tools/list (JSON-RPC 2.0, method "tools/list")。
// mark3labs/mcp-go v0.57.0 を使い、internal/mcp.NewServer が組み立てる本番同等の
// ToolRegistry (backlog.NewMockClient) に対して HandleMessage で直接リクエストを
// 送って取得した (ネットワーク/トランスポート層を経由しない、プロトコルハンドラ直叩き)。
//
// 正規化規則:
//  1. キー順ソート: JSON オブジェクトの全キーをコード順にソートする
//     (encoding/json は map[string]any を marshal する際に自動でキーをソートするため、
//     一度 map[string]any へ decode してから re-encode するだけで達成できる)。
//  2. ツール配列の名前順ソート: result.tools 配列を tool.name の昇順で並べ替える。
//  3. optional field 省略と null の同一視: JSON 値が null のキーは出力から取り除く。
//     mark3labs/mcp-go 側の実装差異で "省略" と "null" が揺れても同一の正規形になる。
//  4. required 配列のソート: 各ツールの inputSchema.required 配列を文字列昇順で
//     ソートする。
//  5. annotation のポインタ値の値化: ToolAnnotation の *bool ヒントは JSON 上では
//     素の true/false リテラルとして表現される (Go 特有のポインタという概念は
//     JSON テキストには残らない)ため、規則1・3を適用した時点で自動的に達成される。
func normalizeToolsListResponse(raw []byte) ([]byte, error) {
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	normalized := dropNullFields(parsed)
	sortToolsAndRequired(normalized)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(normalized); err != nil {
		return nil, err
	}
	// json.Encoder.Encode は末尾に改行を付与するので、そのまま安定した golden 出力として扱う。
	return buf.Bytes(), nil
}

// dropNullFields は JSON 値を再帰的に走査し、オブジェクトのフィールドのうち値が null の
// ものを取り除く。フィールド省略と null を同一視するための正規化。
func dropNullFields(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			if vv == nil {
				continue
			}
			out[k] = dropNullFields(vv)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = dropNullFields(vv)
		}
		return out
	default:
		return val
	}
}

// sortToolsAndRequired は result.tools 配列を name 昇順に、各ツールの
// inputSchema.required 配列を文字列昇順に、in-place でソートする。
func sortToolsAndRequired(v any) {
	root, ok := v.(map[string]any)
	if !ok {
		return
	}
	result, ok := root["result"].(map[string]any)
	if !ok {
		return
	}
	toolsRaw, ok := result["tools"].([]any)
	if !ok {
		return
	}

	for _, toolRaw := range toolsRaw {
		tool, ok := toolRaw.(map[string]any)
		if !ok {
			continue
		}
		inputSchema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			continue
		}
		requiredRaw, ok := inputSchema["required"].([]any)
		if !ok {
			continue
		}
		sort.Slice(requiredRaw, func(i, j int) bool {
			si, _ := requiredRaw[i].(string)
			sj, _ := requiredRaw[j].(string)
			return si < sj
		})
	}

	sort.Slice(toolsRaw, func(i, j int) bool {
		ti, _ := toolsRaw[i].(map[string]any)
		tj, _ := toolsRaw[j].(map[string]any)
		ni, _ := ti["name"].(string)
		nj, _ := tj["name"].(string)
		return ni < nj
	})
	result["tools"] = toolsRaw
}

// TestNormalizeToolsListResponse_Idempotent は正規化器が冪等であることを確認する。
// baseline を正規化した結果を再度正規化しても、バイト列として完全に一致しなければならない。
// これにより baseline ファイル自体が既に正規形であることも同時に保証する。
func TestNormalizeToolsListResponse_Idempotent(t *testing.T) {
	baseline, err := os.ReadFile("testdata/tools_list_baseline.json")
	if err != nil {
		t.Fatalf("failed to read baseline: %v", err)
	}

	once, err := normalizeToolsListResponse(baseline)
	if err != nil {
		t.Fatalf("normalizeToolsListResponse(baseline) error: %v", err)
	}
	if !bytes.Equal(once, baseline) {
		t.Fatalf("baseline is not already in normal form; normalize(baseline) != baseline")
	}

	twice, err := normalizeToolsListResponse(once)
	if err != nil {
		t.Fatalf("normalizeToolsListResponse(once) error: %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Fatalf("normalizer is not idempotent: normalize(x) != normalize(normalize(x))")
	}
}

// TestNormalizeToolsListResponse_SortsToolsByName は入力のツール順が乱れていても
// name 昇順に整列されることを確認する。
func TestNormalizeToolsListResponse_SortsToolsByName(t *testing.T) {
	raw := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"tools": [
				{"name": "logvalet_zzz", "description": null, "inputSchema": {"type": "object", "properties": {}, "required": []}},
				{"name": "logvalet_aaa", "inputSchema": {"type": "object", "properties": {}, "required": []}}
			]
		}
	}`)
	got, err := normalizeToolsListResponse(raw)
	if err != nil {
		t.Fatalf("normalizeToolsListResponse error: %v", err)
	}

	var parsed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("failed to parse normalized output: %v", err)
	}
	if len(parsed.Result.Tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2", len(parsed.Result.Tools))
	}
	if parsed.Result.Tools[0].Name != "logvalet_aaa" || parsed.Result.Tools[1].Name != "logvalet_zzz" {
		t.Errorf("tools order = [%s, %s], want [logvalet_aaa, logvalet_zzz]",
			parsed.Result.Tools[0].Name, parsed.Result.Tools[1].Name)
	}
	// description: null は省略されたキーと同一視されるため、"description" キー自体が
	// 出力から消えていることを確認する。
	if bytes.Contains(got, []byte(`"description"`)) {
		t.Errorf("normalized output should not contain a null-valued \"description\" key: %s", got)
	}
}

// TestNormalizeToolsListResponse_SortsRequiredArray は required 配列が文字列昇順に
// 整列されることを確認する。
func TestNormalizeToolsListResponse_SortsRequiredArray(t *testing.T) {
	raw := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"tools": [
				{
					"name": "logvalet_issue_update",
					"inputSchema": {
						"type": "object",
						"properties": {},
						"required": ["project_id", "issue_key"]
					}
				}
			]
		}
	}`)
	got, err := normalizeToolsListResponse(raw)
	if err != nil {
		t.Fatalf("normalizeToolsListResponse error: %v", err)
	}

	var parsed struct {
		Result struct {
			Tools []struct {
				InputSchema struct {
					Required []string `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("failed to parse normalized output: %v", err)
	}
	required := parsed.Result.Tools[0].InputSchema.Required
	if len(required) != 2 || required[0] != "issue_key" || required[1] != "project_id" {
		t.Errorf("required = %v, want [issue_key project_id]", required)
	}
}

// TestNormalizeToolsListResponse_BaselineToolCount は baseline に含まれるツール数が
// 移行前の登録ツール総数 (72) と一致することを確認する。
// この数は server_test.go の TestNewServerWithFactory_RegistersAllTools が期待する
// expectedCount と同期させる必要がある。
func TestNormalizeToolsListResponse_BaselineToolCount(t *testing.T) {
	baseline, err := os.ReadFile("testdata/tools_list_baseline.json")
	if err != nil {
		t.Fatalf("failed to read baseline: %v", err)
	}
	var parsed struct {
		Result struct {
			Tools []json.RawMessage `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(baseline, &parsed); err != nil {
		t.Fatalf("failed to parse baseline: %v", err)
	}
	const wantCount = 72
	if len(parsed.Result.Tools) != wantCount {
		t.Errorf("baseline tool count = %d, want %d", len(parsed.Result.Tools), wantCount)
	}
}
