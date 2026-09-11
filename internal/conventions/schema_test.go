package conventions

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoad_MinimalYAML(t *testing.T) {
	data := []byte("schema_version: 1\nproject:\n  key: DEMO\n")

	c, err := Load(data)
	if err != nil {
		t.Fatalf("最小 YAML のロードに失敗しました: %v", err)
	}
	if c.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", c.SchemaVersion, SchemaVersion)
	}
	if c.Project.Key != "DEMO" {
		t.Fatalf("project.key = %q, want %q", c.Project.Key, "DEMO")
	}
}

func TestLoad_FullYAMLRoundTrip(t *testing.T) {
	data := []byte(`schema_version: 1
project:
  key: DEMO
  name: デモプロジェクト
priority:
  high: 今すぐ進める
  normal: 計画的に進める
  low: 保留する
close_policy:
  low_untouched_days: 30
statuses:
  - name: 保留
    color: "#ea2c00"
issue_types:
  - name: 規約
    color: "#e30000"
    template_summary: "規約: 要約"
    template_description: |
      規約の説明です。
      2 行目です。
  - name: 案件
    color: "#990000"
initiatives:
  - name: 重点テーマ
    description: 数か月規模のテーマ
engagements:
  - name: 顧客基盤更改
    lead: alice
    initiative: 重点テーマ
    start_date: "2026-09-01"
    due_date: "2026-09-30"
`)

	want, err := Load(data)
	if err != nil {
		t.Fatalf("フル YAML のロードに失敗しました: %v", err)
	}

	encoded, err := yaml.Marshal(want)
	if err != nil {
		t.Fatalf("YAML の再エンコードに失敗しました: %v", err)
	}
	got, err := Load(encoded)
	if err != nil {
		t.Fatalf("再エンコードした YAML のロードに失敗しました: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ラウンドトリップ結果が一致しません:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLoad_LowUntouchedDaysDistinguishesUnsetAndZero(t *testing.T) {
	unset, err := Load([]byte("schema_version: 1\nclose_policy: {}\n"))
	if err != nil {
		t.Fatalf("未指定値のロードに失敗しました: %v", err)
	}
	if unset.ClosePolicy.LowUntouchedDays != nil {
		t.Fatalf("未指定値 = %v, want nil", unset.ClosePolicy.LowUntouchedDays)
	}

	zero, err := Load([]byte("schema_version: 1\nclose_policy:\n  low_untouched_days: 0\n"))
	if err != nil {
		t.Fatalf("0 のロードに失敗しました: %v", err)
	}
	if zero.ClosePolicy.LowUntouchedDays == nil || *zero.ClosePolicy.LowUntouchedDays != 0 {
		t.Fatalf("0 の明示指定 = %v, want pointer to 0", zero.ClosePolicy.LowUntouchedDays)
	}
}
