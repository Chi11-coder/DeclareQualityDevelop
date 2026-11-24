package dqd

import (
	"strings"
	"testing"
)

const valid = `---
id: TICKET-001
title: 画面の変更
domains: [frontend]
attributes: []
quality:
  include: [UI-001]
---

# 機能要件

機能の説明。

## 操作

入力できること。

# 個別合格条件

## UI-001

375pxで確認する。
`

// TestParseTicketWithBOMAndCRLF はBOMとCRLFを含むチケットでも、機能要件と個別合格条件が失われないことを検証する。
func TestParseTicketWithBOMAndCRLF(t *testing.T) {
	task, err := ParseTicket([]byte("\ufeff"+strings.ReplaceAll(valid, "\n", "\r\n")), "TICKET-001")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "TICKET-001" || !strings.Contains(task.Functional, "## 操作") || task.Criteria["UI-001"] != "375pxで確認する。" {
		t.Fatalf("情報が失われています: %+v", task)
	}
}

// TestRejectMalformedTicket は不正なFront Matterや本文構造を持つチケットを拒否することを検証する。
func TestRejectMalformedTicket(t *testing.T) {
	tests := map[string]string{
		"閉じていないコード": valid + "\n```markdown\n本文\n",
		"ID不一致":     strings.Replace(valid, "id: TICKET-001", "id: TICKET-002", 1),
		"未知フィールド":   strings.Replace(valid, "domains:", "domain:", 1),
		"数値タイトル":    strings.Replace(valid, "title: 画面の変更", "title: 123", 1),
		"型違い":       strings.Replace(valid, "attributes: []", "attributes: true", 1),
		"YAML重複":    strings.Replace(valid, "title: 画面の変更", "title: A\ntitle: B", 1),
		"重複属性":      strings.Replace(valid, "attributes: []", "attributes: [screen, screen]", 1),
		"機能欠落":      strings.Replace(valid, "# 機能要件", "# 別の見出し", 1),
		"本文の取りこぼし":  strings.Replace(valid, "# 機能要件", "読み落としてはいけない本文\n\n# 機能要件", 1),
		"重複合格条件":    valid + "\n## UI-001\n別の条件\n",
		"空の合格条件":    strings.Replace(valid, "375pxで確認する。", "", 1),
		"合格条件の前文":   strings.Replace(valid, "# 個別合格条件", "# 個別合格条件\nここに要求を書かない", 1),
	}
	for name, text := range tests {
		// 各不正形式を個別のサブテストとして確認する。
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTicket([]byte(text), "TICKET-001"); err == nil {
				t.Fatal("不正なチケットが受理されました")
			}
		})
	}
}
