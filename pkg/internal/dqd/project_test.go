package dqd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestProjectReadRejectsOversizedInput は上限を超える入力がエラーになることを確認する。
func TestProjectReadRejectsOversizedInput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "requirements.yml")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	if err := file.Truncate(maxProjectInputBytes + 1); err != nil {
		t.Fatal(err)
	}
	project, err := OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	defer project.Close()
	if _, err := project.Read("requirements.yml"); err == nil || !strings.Contains(err.Error(), "4 MiB") {
		t.Fatalf("サイズ制限のエラーが必要です: %v", err)
	}
}

// TestProjectReadRejectsNamedPipe は名前付きパイプを読み込み待ちせず拒否することを確認する。
func TestProjectReadRejectsNamedPipe(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifoがありません")
	}
	root := t.TempDir()
	if err := exec.Command(mkfifo, filepath.Join(root, "ticket.md")).Run(); err != nil {
		t.Fatal(err)
	}
	project, err := OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	defer project.Close()
	result := make(chan error, 1)
	go func() {
		_, err := project.Read("ticket.md")
		result <- err
	}()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "通常ファイル") {
			t.Fatalf("通常ファイル以外を拒否するエラーが必要です: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("名前付きパイプの読み込みが待ち続けています")
	}
}
