package conventions

import "testing"

func TestGlossary(t *testing.T) {
	got := Glossary()
	want := []GlossaryEntry{
		{Term: "conventions", Meaning: "組織の運用規約。Linear の制約を Backlog の語彙に翻訳し、各項目の意味を自分たちの言葉で書いたもの", BacklogForm: "規約課題（入力は conventions.yaml）"},
		{Term: "Initiative", Meaning: "数か月規模の重点テーマ。並び順が優先度。案件は必ずいずれかに属する", BacklogForm: "なし（conventions 内のリスト）"},
		{Term: "案件", Meaning: "数週間規模の取り組み。Linear の Project に相当", BacklogForm: "カテゴリ + 種別「案件」の親課題"},
		{Term: "Lead", Meaning: "案件の責任者。1 人だけ", BacklogForm: "案件親課題の担当者"},
		{Term: "案件親課題", Meaning: "案件のヘッダーとなる課題。Lead・期間・状態・Context & Goals を持つ", BacklogForm: "種別「案件」の課題"},
		{Term: "規約課題", Meaning: "規約の正本。説明欄に運用ガイドと YAML を持ち、変更履歴とコメントで規約の議論を残す", BacklogForm: "種別「規約」の課題（プロジェクトに 1 件）"},
		{Term: "曖昧さ", Meaning: "規約に照らして決まっていないこと。案件不明の課題、Lead 不在の案件など", BacklogForm: "health の ambiguities"},
	}
	if len(got) != len(want) {
		t.Fatalf("用語数 = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("用語 %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
