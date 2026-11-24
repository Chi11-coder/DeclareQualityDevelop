package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run はCLIを指定した引数で実行し、終了コードと出力先の使い分けを検証する。
func run(t *testing.T, want int, args ...string) (string, string) {
	t.Helper()
	return runWithInput(t, want, "", args...)
}

// runWithInput は標準入力を指定してCLIを実行し、終了コードと出力先を検証する。
func runWithInput(t *testing.T, want int, input string, args ...string) (string, string) {
	t.Helper()
	var out, errors bytes.Buffer
	if code := Run(args, strings.NewReader(input), &out, &errors); code != want {
		t.Fatalf("Run(%q) = %d, want %d\nstdout: %s\nstderr: %s", args, code, want, out.String(), errors.String())
	}
	if want == 0 && errors.Len() != 0 {
		t.Fatalf("成功時のstderr: %s", errors.String())
	}
	if want != 0 && errors.Len() == 0 {
		t.Fatal("エラーの説明がありません")
	}
	return out.String(), errors.String()
}

// read は検証対象のファイル内容を読み込み、失敗時はテストを中断する。
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// write は必要な親ディレクトリを作成し、検証用のファイルを書き込む。
func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

// replace は検証用ファイルに置換対象が存在することを確認して、一か所だけ変更する。
func replace(t *testing.T, path, before, after string) {
	t.Helper()
	text := read(t, path)
	if !strings.Contains(text, before) {
		t.Fatalf("置換対象がありません: %q", before)
	}
	write(t, path, strings.Replace(text, before, after, 1))
}

// example はTODOの入力例を一時ディレクトリへコピーし、元のexampleを変更せず検証できる状態にする。
func example(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join("..", "..", "..", "docs", "example")
	for _, name := range []string{"requirements.yml", "quality", "tickets"} {
		// 宣言とチケットだけを検証用の一時ディレクトリへコピーする。
		err := filepath.WalkDir(filepath.Join(source, name), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			write(t, filepath.Join(root, rel), read(t, path))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// absent は指定したパスが生成されていないことを確認する。
func absent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("生成してはいけないパスが存在します: %s (%v)", path, err)
	}
}

// TestTodoExampleReadOnlyCommands は全チケットの要求選択とプレビューを確認し、読み取り操作で成果物が作られないことを検証する。
func TestTodoExampleReadOnlyCommands(t *testing.T) {
	root := example(t)
	out, _ := run(t, 0, "-C", root, "validate")
	if !strings.Contains(out, "5 requirements, 3 tickets") {
		t.Fatal(out)
	}
	for number := 1; number <= 3; number++ {
		id := fmt.Sprintf("TICKET-%03d", number)
		out, _ := run(t, 0, "resolve", "-C", root, "-json", id)
		var resolved []struct {
			Requirement struct{ ID string }
			Reason      string
		}
		if err := json.Unmarshal([]byte(out), &resolved); err != nil {
			t.Fatal(err)
		}
		if len(resolved) != 3 || resolved[0].Requirement.ID != "CM-001" || resolved[1].Requirement.ID != "UI-001" || resolved[2].Requirement.ID != fmt.Sprintf("FE-%03d", number) {
			t.Fatalf("要求選択が不正です: %s", out)
		}
		out, _ = run(t, 0, "context", "-C", root, "-stdout", id)
		for _, part := range []string{"# Development Context", id, "### CM-001：", "### UI-001：", fmt.Sprintf("### FE-%03d：", number), "#### 共通合格基準", "#### 個別合格条件", "### 前提", "375px"} {
			if !strings.Contains(out, part) {
				t.Errorf("%sのContextに%qがありません", id, part)
			}
		}
		for other := 1; other <= 3; other++ {
			if other != number && strings.Contains(out, fmt.Sprintf("### FE-%03d：", other)) {
				t.Fatal("未適用の要求がContextに混入しています")
			}
		}
	}
	list, _ := run(t, 0, "quality", "list", "-C", root, "-json")
	if !json.Valid([]byte(list)) {
		t.Fatal(list)
	}
	detail, _ := run(t, 0, "quality", "show", "-C", root, "FE-001")
	if !strings.HasPrefix(detail, "## FE-001：") || strings.Contains(detail, "## FE-002：") {
		t.Fatal(detail)
	}
	for _, name := range []string{".dqd", "app", "contexts", "tests"} {
		absent(t, filepath.Join(root, name))
	}
}

// TestContextOverwriteRequiresForceAndErrorsPreservePreviousFile は明示的な上書き指定と、生成失敗時の既存Contextの保護を検証する。
func TestContextOverwriteRequiresForceAndErrorsPreservePreviousFile(t *testing.T) {
	root := example(t)
	run(t, 0, "context", "-C", root, "TICKET-001")
	path := filepath.Join(root, ".dqd", "contexts", "TICKET-001.md")
	first := read(t, path)
	write(t, path, "利用者が編集したContext\n")
	run(t, 1, "context", "-C", root, "TICKET-001")
	if got := read(t, path); got != "利用者が編集したContext\n" {
		t.Fatal("既存ファイルが上書きされました")
	}
	run(t, 0, "context", "-C", root, "-force", "TICKET-001")
	if got := read(t, path); got != first {
		t.Fatal("同じ入力の生成結果が変化しています")
	}
	replace(t, filepath.Join(root, "quality", "frontend.md"), "## UI-001：", "## MISSING：")
	run(t, 1, "context", "-C", root, "-force", "TICKET-001")
	if read(t, path) != first {
		t.Fatal("生成失敗で既存のContextが変更されました")
	}
	temps, err := filepath.Glob(filepath.Join(root, ".dqd", "contexts", ".dqd-*.tmp"))
	if err != nil || len(temps) != 0 {
		t.Fatalf("一時ファイルが残っています: %v, %v", temps, err)
	}
}

// TestInitAndNewTicket は初期化とチケット雛形生成の一連の動作、既存ファイルの保護を検証する。
func TestInitAndNewTicket(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "new-project")
	run(t, 0, "init", "-C", parent, "-name", "new-project")
	run(t, 0, "validate", "-C", root)
	run(t, 0, "ticket", "new", "-C", root, "-title", "設定を変更する", "-domains", "frontend,backend", "-include", "CM-001", "TICKET-001")
	path := filepath.Join(root, "tickets", "TICKET-001.md")
	text := read(t, path)
	if !strings.Contains(text, "設定を変更する") || !strings.Contains(text, "backend") {
		t.Fatal(text)
	}
	run(t, 0, "validate", "-C", root)
	run(t, 1, "ticket", "new", "-C", root, "TICKET-001")
	if read(t, path) != text {
		t.Fatal("既存のチケットを変更しました")
	}
	run(t, 1, "ticket", "new", "-C", root, "-include", "UNKNOWN", "TICKET-002")
	absent(t, filepath.Join(root, "tickets", "TICKET-002.md"))
	run(t, 1, "init", "-C", parent, "-name", "new-project")
	if read(t, path) != text {
		t.Fatal("再初期化でチケットが変更されました")
	}
	absent(t, filepath.Join(root, ".dqd"))
}

// TestInitPreflightsExistingDocuments は既存の名称フォルダを拒否し、文書や定義ファイルを変更しないことを検証する。
func TestInitPreflightsExistingDocuments(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "new-project")
	write(t, filepath.Join(root, "quality", "common.md"), "利用者の文書")
	run(t, 1, "init", "-C", parent, "-name", "new-project")
	absent(t, filepath.Join(root, "requirements.yml"))
	if read(t, filepath.Join(root, "quality", "common.md")) != "利用者の文書" {
		t.Fatal("利用者の文書を変更しました")
	}
}

// TestNoApplicableQualityCannotGenerateContext は適用要求がないチケットを拒否し、Contextを生成しないことを検証する。
func TestNoApplicableQualityCannotGenerateContext(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "new-project")
	run(t, 0, "init", "-C", parent, "-name", "new-project")
	replace(t, filepath.Join(root, "requirements.yml"), "mode: always", "mode: explicit")
	write(t, filepath.Join(root, "tickets", "TICKET-001.md"), "---\nid: TICKET-001\ntitle: 要求の指定漏れ\nquality:\n  include: []\n---\n\n# 機能要件\n\n機能を追加する。\n\n# 個別合格条件\n")
	run(t, 1, "validate", "-C", root)
	_, message := run(t, 1, "context", "-C", root, "TICKET-001")
	if !strings.Contains(message, "適用する品質要求がありません") {
		t.Fatal(message)
	}
	absent(t, filepath.Join(root, ".dqd"))
}

// TestInvalidDeclarationsCannotGenerateContext は不正なYAML・品質ID・参照先・見出しを拒否し、成果物を作成しないことを検証する。
func TestInvalidDeclarationsCannotGenerateContext(t *testing.T) {
	tests := []struct{ name, file, before, after string }{
		{"schema", "requirements.yml", "schema_version: 1", "schema_version: 2"},
		{"unknown field", "requirements.yml", "characteristic:", "characterstic:"},
		{"duplicate ID", "requirements.yml", "id: FE-001", "id: UI-001"},
		{"duplicate YAML key", "requirements.yml", "schema_version: 1", "schema_version: 1\nschema_version: 1"},
		{"numeric name", "requirements.yml", "name: 既存機能の維持", "name: 123"},
		{"unknown include", "tickets/TICKET-001.md", "- FE-001", "- UNKNOWN"},
		{"unselected criteria", "tickets/TICKET-001.md", "## FE-001", "## FE-003"},
		{"duplicate section", "quality/frontend.md", "## FE-001：", "## UI-001："},
		{"missing section", "quality/frontend.md", "## UI-001：", "## UI-0010："},
		{"external path", "requirements.yml", "path: quality/common.md", "path: ../outside.md"},
		{"manual verification", "requirements.yml", "type: automated", "type: manual"},
		{"deep requirement heading", "quality/frontend.md", "### 品質要求", "###### 品質要求"},
		{"deep criteria heading", "tickets/TICKET-001.md", "## FE-001\n", "## FE-001\n\n##### 深い見出し\n"},
		{"multiple attributes", "requirements.yml", "mode: explicit", "mode: automatic\n      attributes: [screen, input_form]"},
		{"multiple YAML documents", "requirements.yml", "schema_version: 1", "schema_version: 1\n---\nschema_version: 1"},
	}
	for _, test := range tests {
		// 不正な宣言の種類ごとに、独立したプロジェクトで拒否を確認する。
		t.Run(test.name, func(t *testing.T) {
			root := example(t)
			replace(t, filepath.Join(root, test.file), test.before, test.after)
			run(t, 1, "validate", "-C", root)
			run(t, 1, "context", "-C", root, "TICKET-001")
			absent(t, filepath.Join(root, ".dqd"))
		})
	}
}

// TestSymlinksCannotEscapeProjectOrRedirectOutput はシンボリックリンクによるプロジェクト外の参照と出力先の変更を拒否することを検証する。
func TestSymlinksCannotEscapeProjectOrRedirectOutput(t *testing.T) {
	// 内部を指すリンクでも、出力先の差し替えを拒否する。
	t.Run("internal output symlink", func(t *testing.T) {
		root := example(t)
		if err := os.Symlink("quality", filepath.Join(root, ".dqd")); err != nil {
			t.Skip(err)
		}
		run(t, 1, "context", "-C", root, "-force", "TICKET-001")
		absent(t, filepath.Join(root, "quality", "contexts"))
	})
	// 外部ファイルを指す入力リンクを読み込めないことを確認する。
	t.Run("input", func(t *testing.T) {
		root, outside := example(t), t.TempDir()
		secret := filepath.Join(outside, "external.md")
		write(t, secret, "## CM-001：外部\n外部の本文\n")
		if err := os.Remove(filepath.Join(root, "quality", "common.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(secret, filepath.Join(root, "quality", "common.md")); err != nil {
			t.Skip(err)
		}
		run(t, 1, "context", "-C", root, "-stdout", "TICKET-001")
		absent(t, filepath.Join(root, ".dqd"))
	})
	// 外部ディレクトリを指すリンクへ出力しないことを確認する。
	t.Run("output", func(t *testing.T) {
		root, outside := example(t), t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, ".dqd")); err != nil {
			t.Skip(err)
		}
		run(t, 1, "context", "-C", root, "-force", "TICKET-001")
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatal("プロジェクト外に出力しました", entries, err)
		}
	})
}

// TestHelpUsageAndUnknownCommands はヘルプ、引数不足、未知のコマンド、パスを含むIDの扱いを検証する。
func TestHelpUsageAndUnknownCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"-h"}, {"init", "-h"}, {"context", "-h"}, {"quality", "-h"}, {"ticket", "new", "-h"}, {"verify", "-h"}, {"report", "-h"}} {
		out, _ := run(t, 0, args...)
		if !strings.Contains(out, "Usage:") {
			t.Fatal(out)
		}
	}
	for _, args := range [][]string{
		{"unknown"}, {"verify"}, {"report"}, {"verify", "-timeout=0", "TICKET-001"}, {"report", "-format=csv", "TICKET-001"},
		{"context"}, {"resolve", "-missing"}, {"context", "TICKET-001", "-stdout"}, {"validate", "extra"},
	} {
		run(t, 2, args...)
	}
	root := example(t)
	run(t, 1, "context", "-C", root, "../escape")
	absent(t, filepath.Join(root, ".dqd"))
}
