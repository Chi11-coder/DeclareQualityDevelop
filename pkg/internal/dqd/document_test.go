package dqd

import (
	"strings"
	"testing"
)

// TestRequirementBoundariesAndCodeFences は類似IDや隣の要求を含めず、コード内の見出しを保持して抽出・変換できることを検証する。
func TestRequirementBoundariesAndCodeFences(t *testing.T) {
	text := "# 品質\n\n## UI-0010：別ID\n含めない\n\n## UI-001：対象\n### 基準\n保持する\n```markdown\n## UI-002：コード内の例\n```\n~~~\n## OTHER\n~~~\n\n## UI-002：次の要求\n含めない\n"
	got, err := ExtractRequirement(text, "UI-001")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "保持する") || !strings.Contains(got, "## UI-002：コード内の例") || strings.Contains(got, "含めない") {
		t.Fatalf("抽出範囲が不正です:\n%s", got)
	}
	shifted, err := ShiftMarkdownHeadings(got, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(shifted, "### UI-001：対象") || !strings.Contains(shifted, "#### 基準") || !strings.Contains(shifted, "\n## UI-002：コード内の例\n") {
		t.Fatalf("コード内の見出しを変更しています:\n%s", shifted)
	}
}

// TestRequirementRejectsMissingDuplicateEmptyAndTooDeep は要求の欠落・重複・空本文・閉じ忘れと、深すぎる見出しを拒否することを検証する。
func TestRequirementRejectsMissingDuplicateEmptyAndTooDeep(t *testing.T) {
	for _, text := range []string{
		"## UI-0010：類似\n内容\n",
		"## UI-001：対象\n内容\n## UI-001:重複\n内容\n",
		"## UI-001：対象\n\n## UI-002：次\n内容\n",
		"## UI-001：対象\n内容\n```markdown\n閉じ忘れ\n",
	} {
		if _, err := ExtractRequirement(text, "UI-001"); err == nil {
			t.Fatalf("エラーが必要です: %q", text)
		}
	}
	if _, err := ShiftMarkdownHeadings("###### 深い見出し\n内容", 1); err == nil {
		t.Fatal("6を超える階層はエラーにする必要があります")
	}
}

// TestTicketContextPreservesIndentedCode は先頭のインデント付きコードがContextで見出しに変わらないことを確認する。
func TestTicketContextPreservesIndentedCode(t *testing.T) {
	text := "---\nid: TICKET-001\ntitle: インデントの確認\n---\n\n# 機能要件\n\n    # コードのコメント\n    command\n\n# 個別合格条件\n\n## CM-001\n\n    # 確認手順\n    test-command\n"
	ticket, err := ParseTicket([]byte(text), "TICKET-001")
	if err != nil {
		t.Fatal(err)
	}
	requirements := []ResolvedRequirement{{Requirement: Requirement{ID: "CM-001"}}}
	details := map[string]string{"CM-001": "## CM-001\n\n確認する"}
	context, err := GenerateContext(ticket, requirements, details)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"    # コードのコメント\n    command", "    # 確認手順\n    test-command"} {
		if !strings.Contains(context, code) {
			t.Errorf("インデント付きコードが保持されていません: %q\n%s", code, context)
		}
	}
}
