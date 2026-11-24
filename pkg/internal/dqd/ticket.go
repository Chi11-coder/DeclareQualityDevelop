package dqd

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// TicketMetadata はチケット先頭のYAMLに記述する項目を表す。
type TicketMetadata struct {
	ID         string   `yaml:"id"`
	Title      string   `yaml:"title"`
	Domains    []string `yaml:"domains"`
	Attributes []string `yaml:"attributes"`
	Quality    struct {
		Include []string `yaml:"include"`
	} `yaml:"quality"`
}

// Ticket は実装する機能と品質IDごとの個別合格条件を保持する。
type Ticket struct {
	TicketMetadata
	Functional string
	Criteria   map[string]string
}

// ParseTicket はFront Matterと本文を解析し、チケットID・機能要件・個別合格条件を検証する。
func ParseTicket(data []byte, expectedID string) (Ticket, error) {
	var result Ticket
	if !utf8.Valid(data) {
		return result, errors.New("チケットはUTF-8で記述してください")
	}
	lines := strings.Split(normalizeMarkdown(string(data)), "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return result, errors.New("チケットの先頭にYAML Front Matterが必要です")
	}
	end := 1
	for end < len(lines) && lines[end] != "---" {
		end++
	}
	if end == len(lines) {
		return result, errors.New("YAML Front Matterを閉じる---がありません")
	}
	if err := decodeDeclaration([]byte(strings.Join(lines[1:end], "\n")), &result.TicketMetadata); err != nil {
		return result, fmt.Errorf("Front Matter: %w", err)
	}
	if !ValidID(result.ID) || result.ID != expectedID {
		return result, fmt.Errorf("ファイル名とチケットIDが一致しないか、IDが不正です: %q", result.ID)
	}
	if err := ValidateText(result.Title, "title"); err != nil {
		return result, err
	}
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"domains", result.Domains}, {"attributes", result.Attributes}, {"quality.include", result.Quality.Include},
	} {
		if err := validateList(field.values, field.name); err != nil {
			return result, err
		}
	}
	body := lines[end+1:]
	bodyHeadings, closed := scanMarkdown(body)
	if !closed {
		return result, errUnclosedFence
	}
	if len(bodyHeadings) > 0 {
		first := bodyHeadings[0]
		if first.level != 1 || strings.TrimSpace(strings.Join(body[:first.line], "\n")) != "" {
			return result, errors.New("本文はレベル1の機能要件または個別合格条件の見出しから記述してください")
		}
	}
	for _, heading := range bodyHeadings {
		if heading.level == 1 && heading.title != "機能要件" && heading.title != "個別合格条件" {
			return result, fmt.Errorf("未対応のレベル1見出しです: %q", heading.title)
		}
	}
	var err error
	result.Functional, err = oneMarkdownSection(body, bodyHeadings, 1, "機能要件")
	if err != nil {
		return result, err
	}
	if result.Functional == "" {
		return result, errors.New("機能要件の本文が空です")
	}
	criteria, err := oneMarkdownSection(body, bodyHeadings, 1, "個別合格条件")
	if err != nil {
		return result, err
	}
	result.Criteria, err = parseTicketCriteria(criteria)
	return result, err
}

// parseTicketCriteria は個別合格条件を品質IDごとの本文に分ける。
func parseTicketCriteria(criteria string) (map[string]string, error) {
	byQualityID := make(map[string]string)
	criteriaLines := strings.Split(criteria, "\n")
	headings, _ := scanMarkdown(criteriaLines)
	first := len(criteriaLines)
	for _, heading := range headings {
		if heading.level != 2 {
			continue
		}
		if first == len(criteriaLines) {
			first = heading.line
		}
		id := heading.title
		if !ValidID(id) {
			return byQualityID, fmt.Errorf("個別合格条件の見出しには品質IDだけを記述してください: %q", id)
		}
		if _, exists := byQualityID[id]; exists {
			return byQualityID, fmt.Errorf("個別合格条件のIDが重複しています: %s", id)
		}
		value := markdownSection(criteriaLines, headings, heading, false)
		if value == "" {
			return byQualityID, fmt.Errorf("%sの個別合格条件が空です", id)
		}
		byQualityID[id] = value
	}
	if strings.TrimSpace(strings.Join(criteriaLines[:first], "\n")) != "" {
		return byQualityID, errors.New("個別合格条件は品質IDのレベル2見出しごとに記述してください")
	}
	return byQualityID, nil
}
