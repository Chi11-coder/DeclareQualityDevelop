package dqd

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var atxHeadingPattern = regexp.MustCompile(`^ {0,3}(#{1,6})[\t ]+(.+?)\s*$`)
var closingHashesPattern = regexp.MustCompile(`[\t ]+#+[\t ]*$`)

var errUnclosedFence = errors.New("Markdownのコードブロックが閉じられていません")

type heading struct {
	line  int
	level int
	title string
}

// normalizeMarkdown はMarkdown先頭のBOMを除去し、CRLFをLFへ揃える。
func normalizeMarkdown(text string) string {
	return strings.ReplaceAll(strings.TrimPrefix(text, "\ufeff"), "\r\n", "\n")
}

// scanMarkdown は行ごとの見出しとコードブロックを走査し、見出し一覧と閉じた状態を返す。
func scanMarkdown(lines []string) ([]heading, bool) {
	var headings []heading
	var fence byte
	var width int
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			n := 0
			for n < len(trimmed) && trimmed[n] == trimmed[0] {
				n++
			}
			if n >= 3 {
				if fence == 0 && (trimmed[0] != '`' || !strings.ContainsRune(trimmed[n:], '`')) {
					fence, width = trimmed[0], n
				} else if trimmed[0] == fence && n >= width && strings.TrimSpace(trimmed[n:]) == "" {
					fence = 0
				}
				continue
			}
		}
		if fence != 0 {
			continue
		}
		if match := atxHeadingPattern.FindStringSubmatch(line); match != nil {
			headings = append(headings, heading{line: i, level: len(match[1]), title: strings.TrimSpace(closingHashesPattern.ReplaceAllString(match[2], ""))})
		}
	}
	return headings, fence == 0
}

// markdownSection は指定した見出しから、次の同じ階層以上の見出しの手前までを抽出する。
func markdownSection(lines []string, headings []heading, startHeading heading, includeHeading bool) string {
	end := len(lines)
	for _, next := range headings {
		if next.line > startHeading.line && next.level <= startHeading.level {
			end = next.line
			break
		}
	}
	start := startHeading.line + 1
	if includeHeading {
		start = startHeading.line
	}
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}

// oneMarkdownSection は指定した階層・名前の見出しが一つだけあることを確認して本文を抽出する。
func oneMarkdownSection(lines []string, headings []heading, level int, title string) (string, error) {
	var found []heading
	for _, heading := range headings {
		if heading.level == level && heading.title == title {
			found = append(found, heading)
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("見出し %q は一つ必要です（検出数: %d）", title, len(found))
	}
	return markdownSection(lines, headings, found[0], false), nil
}

// ExtractRequirement は品質IDと一致する要求セクションを抽出し、欠落・重複・空の本文を拒否する。
func ExtractRequirement(text, id string) (string, error) {
	if !utf8.ValidString(text) {
		return "", errors.New("MarkdownはUTF-8で記述してください")
	}
	lines := strings.Split(normalizeMarkdown(text), "\n")
	headings, closed := scanMarkdown(lines)
	if !closed {
		return "", errUnclosedFence
	}
	var found []heading
	for _, heading := range headings {
		if heading.level == 2 && (heading.title == id || strings.HasPrefix(heading.title, id+"：") || strings.HasPrefix(heading.title, id+":")) {
			found = append(found, heading)
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("品質要求セクション %s は一つ必要です（検出数: %d）", id, len(found))
	}
	if markdownSection(lines, headings, found[0], false) == "" {
		return "", fmt.Errorf("品質要求セクション %s の本文が空です", id)
	}
	return markdownSection(lines, headings, found[0], true), nil
}

// ShiftMarkdownHeadings はコードブロックを保持して見出しを深くし、レベル6を超える変換を拒否する。
func ShiftMarkdownHeadings(text string, delta int) (string, error) {
	lines := strings.Split(text, "\n")
	headings, _ := scanMarkdown(lines)
	prefix := strings.Repeat("#", delta)
	for _, heading := range headings {
		if heading.level+delta > 6 {
			return "", fmt.Errorf("見出し %q の階層がContextでは6を超えます", heading.title)
		}
		lines[heading.line] = prefix + strings.TrimLeft(lines[heading.line], " ")
	}
	return strings.Join(lines, "\n"), nil
}
