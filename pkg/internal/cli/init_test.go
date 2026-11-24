package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInteractiveInitCreatesNamedProject は入力した名前のフォルダ内にだけ初期ファイルを作成することを検証する。
func TestInteractiveInitCreatesNamedProject(t *testing.T) {
	parent := t.TempDir()
	output, _ := runWithInput(t, 0, "todo-app\n", "init", "-C", parent)
	root := filepath.Join(parent, "todo-app")
	if !strings.Contains(output, "プロジェクト名を入力してください:") || !strings.Contains(output, root) {
		t.Fatal(output)
	}
	for _, path := range []string{"requirements.yml", "quality/common.md", "verification.yml", "verifies/.gitignore", "reports/.gitignore"} {
		if read(t, filepath.Join(root, path)) == "" {
			t.Fatalf("初期ファイルが空です: %s", path)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "tickets"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("ticketsは空のディレクトリである必要があります: %v, %v", entries, err)
	}
	absent(t, filepath.Join(parent, "requirements.yml"))
	for _, name := range []string{"app", "tests", "contexts", ".dqd"} {
		absent(t, filepath.Join(root, name))
	}
	run(t, 0, "validate", "-C", root)
}

// TestInteractiveInitRetriesInvalidNames は空入力やパス形式の名前を再入力させ、正常な名前で作成することを検証する。
func TestInteractiveInitRetriesInvalidNames(t *testing.T) {
	parent := t.TempDir()
	input := "\n../escape\nfoo/bar\n todo-app \n"
	output, _ := runWithInput(t, 0, input, "init", "-C", parent)
	if strings.Count(output, "プロジェクト名を入力してください:") != 4 {
		t.Fatal(output)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 || entries[0].Name() != "todo-app" {
		t.Fatalf("想定外のディレクトリが作成されました: %v, %v", entries, err)
	}
}

// TestInitInputEOFDoesNotCreateFiles は入力終了時に処理を中断し、空の親ディレクトリを変更しないことを検証する。
func TestInitInputEOFDoesNotCreateFiles(t *testing.T) {
	for _, input := range []string{"", "\n", "../escape"} {
		parent := t.TempDir()
		runWithInput(t, 1, input, "init", "-C", parent)
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 0 {
			t.Fatalf("中断した初期化でファイルが作成されました: %v, %v", entries, err)
		}
	}
}

// TestInitAcceptsFinalInputWithoutNewline は末尾改行のない有効な名前も読み込めることを検証する。
func TestInitAcceptsFinalInputWithoutNewline(t *testing.T) {
	parent := t.TempDir()
	runWithInput(t, 0, "todo-app", "init", "-C", parent)
	run(t, 0, "validate", "-C", filepath.Join(parent, "todo-app"))
}

// TestNamedInitRejectsUnsafeNames は名前指定による絶対パス・越境・入れ子・長すぎる名前を拒否することを検証する。
func TestNamedInitRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{".", "..", "../escape", "/tmp/escape", "foo/bar", `foo\bar`, "-bad", "two words", strings.Repeat("a", 256)} {
		parent := t.TempDir()
		run(t, 1, "init", "-C", parent, "-name", name)
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 0 {
			t.Fatalf("不正な名前%qで親を変更しました: %v, %v", name, entries, err)
		}
	}
}

// TestInitProtectsExistingDirectoryAndSymlink は既存の空フォルダやリンクを初期化せず、内容を保護することを検証する。
func TestInitProtectsExistingDirectoryAndSymlink(t *testing.T) {
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "existing"), 0755); err != nil {
		t.Fatal(err)
	}
	run(t, 1, "init", "-C", parent, "-name", "existing")
	absent(t, filepath.Join(parent, "existing", "requirements.yml"))
	write(t, filepath.Join(parent, "file"), "既存のファイル")
	run(t, 1, "init", "-C", parent, "-name", "file")
	if read(t, filepath.Join(parent, "file")) != "既存のファイル" {
		t.Fatal("既存ファイルが変更されました")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(parent, "linked")); err != nil {
		t.Skip(err)
	}
	run(t, 1, "init", "-C", parent, "-name", "linked")
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("リンク先が変更されました: %v, %v", entries, err)
	}
}
