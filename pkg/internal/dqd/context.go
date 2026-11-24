package dqd

import (
	"errors"
	"fmt"
	"strings"
)

// GenerateContext は機能要件、適用する品質要求、宣言情報、個別合格条件を開発用Markdownへ結合する。
func GenerateContext(ticket Ticket, requirements []ResolvedRequirement, details map[string]string) (string, error) {
	if len(requirements) == 0 {
		return "", errors.New("適用する品質要求がありません。quality.includeまたは適用条件を確認してください")
	}
	functional, err := ShiftMarkdownHeadings(ticket.Functional, 1)
	if err != nil {
		return "", err
	}
	parts := []string{
		"# Development Context",
		fmt.Sprintf("## Ticket\n\n%s：%s", ticket.ID, ticket.Title),
		fmt.Sprintf("## 参照元\n\n- `tickets/%s.md`\n- `requirements.yml`", ticket.ID),
		"## Functional Requirements\n\n" + functional,
		"## Quality Requirements",
	}
	for _, item := range requirements {
		req := item.Requirement
		detail, exists := details[req.ID]
		if !exists {
			return "", fmt.Errorf("%sの品質要求本文がありません", req.ID)
		}
		body, err := ShiftMarkdownHeadings(detail, 1)
		if err != nil {
			return "", fmt.Errorf("%s: %w", req.ID, err)
		}
		body += fmt.Sprintf("\n\n#### 宣言情報\n\n- 品質ID: %s\n- 技術領域: %s\n- 品質特性: %s\n- 検証: %s / %s\n- 適用理由: %s\n- 参照: `%s` (%s)",
			req.ID, req.Domain, req.Characteristic, req.Verification.Type, req.Verification.Method, item.Reason, req.Document.Path, req.Document.Section)
		if criteria, exists := ticket.Criteria[req.ID]; exists {
			criteria, err = ShiftMarkdownHeadings(criteria, 2)
			if err != nil {
				return "", fmt.Errorf("%sの個別合格条件: %w", req.ID, err)
			}
			body += "\n\n#### 個別合格条件\n\n" + criteria
		}
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n") + "\n", nil
}
