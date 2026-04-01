package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/domain"
)

// TestIssue_JSONTags は Issue の JSON タグが Backlog API (camelCase) に準拠しているかを検証する。
func TestIssue_JSONTags(t *testing.T) {
	now := time.Now()
	issue := domain.Issue{
		ID:          1,
		ProjectID:   100,
		IssueKey:    "PROJ-1",
		Summary:     "テスト課題",
		Description: "説明",
		IssueType:   &domain.IDName{ID: 1, Name: "バグ"},
		Reporter:    &domain.User{ID: 1, Name: "テストユーザー"},
		DueDate:     &now,
		StartDate:   &now,
	}

	b, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	// Backlog API は camelCase を使用する
	cases := []struct {
		field    string
		wantKey  string
		wantPresent bool
	}{
		{"ProjectID", "projectId", true},
		{"IssueType", "issueType", true},
		{"Reporter(createdUser)", "createdUser", true},
		{"DueDate", "dueDate", true},
		{"StartDate", "startDate", true},
	}

	for _, tc := range cases {
		_, ok := m[tc.wantKey]
		if tc.wantPresent && !ok {
			t.Errorf("フィールド %s: JSON キー %q が存在しない（camelCase に修正が必要）", tc.field, tc.wantKey)
		}
	}

	// snake_case キーが存在しないことを確認
	badKeys := []string{"project_id", "issue_type", "created_user", "due_date", "start_date"}
	for _, k := range badKeys {
		if _, ok := m[k]; ok {
			t.Errorf("snake_case キー %q が存在する（camelCase に修正が必要）", k)
		}
	}
}

// TestDocument_JSONTags は Document の JSON タグが Backlog API (camelCase) に準拠しているかを検証する。
func TestDocument_JSONTags(t *testing.T) {
	doc := domain.Document{
		ID:          "test-doc-001",
		ProjectID:   100,
		Title:       "テストドキュメント",
		CreatedUser: &domain.User{ID: 1, Name: "テストユーザー"},
	}

	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	if _, ok := m["projectId"]; !ok {
		t.Error("Document.ProjectID の JSON キーが projectId でない")
	}
	if _, ok := m["createdUser"]; !ok {
		t.Error("Document.CreatedUser の JSON キーが createdUser でない")
	}
	if _, ok := m["project_id"]; ok {
		t.Error("Document.ProjectID に snake_case キー project_id が存在する")
	}
}

// TestUser_NulabAccount は User 型に NulabAccount フィールドが存在するかを検証する。
func TestUser_NulabAccount(t *testing.T) {
	user := domain.User{
		ID:     12345,
		UserID: "naoto",
		Name:   "Naoto Ishizawa",
		NulabAccount: &domain.NulabAccount{
			NulabID: "xxxxx",
		},
	}

	b, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	nulab, ok := m["nulabAccount"]
	if !ok {
		t.Fatal("User に nulabAccount フィールドが存在しない")
	}

	nulabMap, ok := nulab.(map[string]interface{})
	if !ok {
		t.Fatal("nulabAccount が map[string]interface{} でない")
	}

	if _, ok := nulabMap["nulabId"]; !ok {
		t.Error("nulabAccount に nulabId フィールドが存在しない")
	}
}

// TestUserRef_JSONShape は UserRef が spec §11 の simplified form に準拠しているかを検証する。
func TestUserRef_JSONShape(t *testing.T) {
	ref := domain.UserRef{
		ID:   12345,
		Name: "Naoto Ishizawa",
	}

	b, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	if _, ok := m["id"]; !ok {
		t.Error("UserRef に id フィールドが存在しない")
	}
	if _, ok := m["name"]; !ok {
		t.Error("UserRef に name フィールドが存在しない")
	}
}

// TestNormalizedActivity_JSONShape は NormalizedActivity が spec §12 の shape に準拠しているかを検証する。
func TestNormalizedActivity_JSONShape(t *testing.T) {
	now := time.Now()
	act := domain.NormalizedActivity{
		ID:      1001,
		Type:    "issue_commented",
		Created: &now,
		Actor: &domain.UserRef{
			ID:   12345,
			Name: "Naoto Ishizawa",
		},
		Issue: &domain.ActivityIssueRef{
			ID:      555,
			Key:     "PROJ-123",
			Summary: "Login UI bug",
		},
		Comment: &domain.ActivityCommentRef{
			ID:      888,
			Content: "Safari reproduces this.",
		},
	}

	b, err := json.Marshal(act)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	for _, key := range []string{"id", "type", "created", "actor", "issue", "comment"} {
		if _, ok := m[key]; !ok {
			t.Errorf("NormalizedActivity に %q フィールドが存在しない", key)
		}
	}

	// actor shape
	actor, _ := m["actor"].(map[string]interface{})
	if _, ok := actor["id"]; !ok {
		t.Error("actor に id フィールドが存在しない")
	}
	if _, ok := actor["name"]; !ok {
		t.Error("actor に name フィールドが存在しない")
	}

	// issue shape
	issue, _ := m["issue"].(map[string]interface{})
	if _, ok := issue["key"]; !ok {
		t.Error("issue に key フィールドが存在しない")
	}
}

// TestWarning_JSONShape は Warning が spec §9 の warning envelope に準拠しているかを検証する。
func TestWarning_JSONShape(t *testing.T) {
	w := domain.Warning{
		Code:      "project_custom_fields_fetch_failed",
		Message:   "Failed to fetch custom fields.",
		Component: "project.custom_fields",
		Retryable: true,
	}

	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	for _, key := range []string{"code", "message", "component", "retryable"} {
		if _, ok := m[key]; !ok {
			t.Errorf("Warning に %q フィールドが存在しない", key)
		}
	}
}

// TestErrorEnvelope_JSONShape は ErrorEnvelope が spec §9 の error envelope に準拠しているかを検証する。
func TestErrorEnvelope_JSONShape(t *testing.T) {
	env := domain.ErrorEnvelope{
		SchemaVersion: "1",
		Error: domain.ErrorDetail{
			Code:      "issue_not_found",
			Message:   "Issue PROJ-999 was not found.",
			Retryable: false,
		},
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	if _, ok := m["schema_version"]; !ok {
		t.Error("ErrorEnvelope に schema_version フィールドが存在しない")
	}
	if _, ok := m["error"]; !ok {
		t.Fatal("ErrorEnvelope に error フィールドが存在しない")
	}

	errMap, _ := m["error"].(map[string]interface{})
	for _, key := range []string{"code", "message", "retryable"} {
		if _, ok := errMap[key]; !ok {
			t.Errorf("error に %q フィールドが存在しない", key)
		}
	}
}

// TestDigestEnvelope_JSONShape は DigestEnvelope が spec §10 の共通 wrapper shape に準拠しているかを検証する。
func TestDigestEnvelope_JSONShape(t *testing.T) {
	env := domain.DigestEnvelope{
		SchemaVersion: "1",
		Resource:      "issue",
		GeneratedAt:   time.Now(),
		Profile:       "work",
		Space:         "example-space",
		BaseURL:       "https://example-space.backlog.com",
		Warnings:      []domain.Warning{},
		Digest:        map[string]interface{}{"issue": nil},
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}

	for _, key := range []string{"schema_version", "resource", "generated_at", "profile", "space", "base_url", "warnings", "digest"} {
		if _, ok := m[key]; !ok {
			t.Errorf("DigestEnvelope に %q フィールドが存在しない", key)
		}
	}
}

// TestIssueJSON_nullDueDate は Issue の null DueDate が JSON に明示出力されることを検証する。
func TestIssueJSON_nullDueDate(t *testing.T) {
	issue := domain.Issue{ID: 1, IssueKey: "TEST-1", Summary: "test"}
	b, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"dueDate":null`) {
		t.Errorf("null DueDate が出力されない: %s", s)
	}
}

// TestIssueJSON_nullStartDate は Issue の null StartDate が JSON に明示出力されることを検証する。
func TestIssueJSON_nullStartDate(t *testing.T) {
	issue := domain.Issue{ID: 1, IssueKey: "TEST-1", Summary: "test"}
	b, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"startDate":null`) {
		t.Errorf("null StartDate が出力されない: %s", s)
	}
}

// TestDocument_JSONFieldAsObject は Document の json フィールドが JSON オブジェクトとして返された場合に
// 正しくパースできることを検証する（GitHub Issue #1 対応）。
func TestDocument_JSONFieldAsObject(t *testing.T) {
	// Backlog API が json フィールドをオブジェクトとして返すケース
	raw := `{
		"id": "doc-001",
		"projectId": 100,
		"title": "テストドキュメント",
		"json": {"type": "doc", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Hello"}]}]}
	}`

	var doc domain.Document
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v（json フィールドが string 型のため失敗）", err)
	}

	if doc.ID != "doc-001" {
		t.Errorf("ID = %q, want %q", doc.ID, "doc-001")
	}
	if doc.ProjectID != 100 {
		t.Errorf("ProjectID = %d, want %d", doc.ProjectID, 100)
	}
	if doc.Title != "テストドキュメント" {
		t.Errorf("Title = %q, want %q", doc.Title, "テストドキュメント")
	}
	if len(doc.JSON) == 0 {
		t.Error("JSON フィールドが空: オブジェクトが正しくパースされていない")
	}

	// 再シリアライズして json フィールドが保持されることを確認
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}
	jsonField, ok := m["json"]
	if !ok {
		t.Fatal("再シリアライズ後に json フィールドが存在しない")
	}
	// json.RawMessage はオブジェクトとして保持されるはず
	if _, ok := jsonField.(map[string]interface{}); !ok {
		t.Errorf("json フィールドがオブジェクトでない: %T %v", jsonField, jsonField)
	}
}

// TestDocument_JSONFieldAsString は Document の json フィールドが文字列の場合でも
// パースできることを検証する（後方互換性）。
func TestDocument_JSONFieldAsString(t *testing.T) {
	// json フィールドが文字列のケース（後方互換性）
	raw := `{
		"id": "doc-002",
		"projectId": 200,
		"title": "テストドキュメント2",
		"json": "plain text content"
	}`

	var doc domain.Document
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("json.Unmarshal() エラー: %v", err)
	}
	if len(doc.JSON) == 0 {
		t.Error("JSON フィールドが空: 文字列が正しくパースされていない")
	}
}

// TestIssueJSON_withDates は Issue の null でない DueDate/StartDate が正しく出力されることを検証する。
func TestIssueJSON_withDates(t *testing.T) {
	now := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	issue := domain.Issue{ID: 1, IssueKey: "TEST-1", Summary: "test", DueDate: &now, StartDate: &now}
	b, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("json.Marshal() エラー: %v", err)
	}
	s := string(b)
	if strings.Contains(s, `"dueDate":null`) {
		t.Errorf("non-nil DueDate が null になっている: %s", s)
	}
	if strings.Contains(s, `"startDate":null`) {
		t.Errorf("non-nil StartDate が null になっている: %s", s)
	}
}
