package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dqd/internal/dqd"

	"gopkg.in/yaml.v3"
)

// TestVerificationReport は適用対象だけを実行し、未確認を含む結果からHTMLを生成できることを検証する。
func TestVerificationReport(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("DQD_CACHE_DIR", cache)
	root := example(t)
	const id = "TICKET-001"
	run(t, 1, "-C", root, "report", id)
	absent(t, filepath.Join(root, ".dqd"))
	write(t, filepath.Join(root, "frontend", "expected.txt"), "テストの作業ディレクトリ")
	writeVerificationConfig(t, root, []dqd.VerificationCheck{
		{ID: "regression", QualityID: "CM-001", Command: checkCommand(t, "pass"), Directory: "frontend"},
		{ID: "layout-skip", QualityID: "UI-001", Command: checkCommand(t, "skip")},
		{ID: "layout-fail", QualityID: "UI-001", Command: checkCommand(t, "fail")},
		{ID: "unrelated", QualityID: "FE-002", Command: checkCommand(t, "mark")},
	})
	run(t, 0, "-C", root, "validate")
	absent(t, filepath.Join(root, ".dqd"))

	out, _ := run(t, 1, "-C", root, "verify", id)
	if !strings.Contains(out, "合格 1 / 不合格 1 / 未確認 1 / 実行エラー 0") {
		t.Fatal(out)
	}
	absent(t, filepath.Join(root, "ran-unrelated"))
	absent(t, filepath.Join(root, ".dqd"))
	out, _ = run(t, 0, "-C", root, "report", "-format=json", id)
	var result dqd.VerificationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Requirements) != 3 {
		t.Fatalf("対象要求: %+v", result.Requirements)
	}
	layout := result.Requirements[1]
	if layout.ID != "UI-001" || len(layout.Checks) != 2 ||
		layout.Checks[0].Status != dqd.StatusUnverified || layout.Checks[1].Status != dqd.StatusFailed {
		t.Fatalf("チェック結果: %+v", layout)
	}
	if !layout.Checks[1].Truncated || len(layout.Checks[1].Output) != 32*1024 {
		t.Fatal("長いログが制限されていません")
	}

	// 保存後に設定を削除しても、レポートは実行時の記録から生成できる。
	if err := os.Remove(filepath.Join(root, "verification.yml")); err != nil {
		t.Fatal(err)
	}
	path, _ := run(t, 0, "-C", root, "report", id)
	path = strings.TrimSpace(path)
	if path != filepath.Join(root, "reports", id+".html") {
		t.Fatalf("HTMLがプロジェクトのreports直下にありません: %q", path)
	}
	jsonPath := filepath.Join(root, "verifies", id+".json")
	read(t, jsonPath)
	for _, folder := range []string{"reports", "verifies"} {
		if policy := read(t, filepath.Join(root, folder, ".gitignore")); !strings.HasSuffix(policy, "*\n!.gitignore\n") {
			t.Fatalf("成果物の除外規則がありません: %s", folder)
		}
	}
	absent(t, filepath.Join(cache, "dqd"))
	summary := read(t, path)
	if !strings.Contains(summary, "layout-fail") || strings.Contains(summary, "外部ログ") || strings.Contains(summary, "コマンド <code>") {
		t.Fatal("既定のHTMLにチェック情報だけが載っていません")
	}
	if runtime.GOOS != "windows" {
		for _, private := range []string{path, jsonPath, filepath.Dir(path)} {
			info, err := os.Stat(private)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm()&0077 != 0 {
				t.Fatalf("他の利用者へのアクセス権限があります: %s %v", private, info.Mode())
			}
		}
	}
	run(t, 1, "-C", example(t), "report", id)

	run(t, 0, "-C", root, "report", "-details", id)
	report := read(t, path)
	for _, want := range []string{id, "UI-001", "不合格", "検証コマンドが登録されていません", "&lt;script&gt;ログ&lt;/script&gt;"} {
		if !strings.Contains(report, want) {
			t.Errorf("HTMLに %q がありません", want)
		}
	}
	if strings.Contains(report, "<script>") || strings.Contains(report, "<img") {
		t.Fatal("ログがHTMLとして埋め込まれています")
	}
	_, css, _ := strings.Cut(report, "<style>")
	css, _, ok := strings.Cut(css, "</style>")
	hash := sha256.Sum256([]byte(css))
	policy := "default-src 'none'; style-src 'sha256-" + base64.StdEncoding.EncodeToString(hash[:]) + "'; base-uri 'none'; form-action 'none'"
	if !ok || !strings.Contains(html.UnescapeString(report), policy) {
		t.Fatal("CSPが固定のスタイルだけを許可していません")
	}

	t.Setenv("DQD_CACHE_DIR", t.TempDir())
	run(t, 0, "-C", root, "report", id)
}

// TestVerifyRerun は全件合格後の再検証で、起動失敗とタイムアウトが記録されることを確認する。
func TestVerifyRerun(t *testing.T) {
	t.Setenv("DQD_CACHE_DIR", t.TempDir())
	root := example(t)
	const id = "TICKET-001"
	write(t, filepath.Join(root, "expected.txt"), "テストの作業ディレクトリ")
	checks := []dqd.VerificationCheck{
		{ID: "regression", QualityID: "CM-001", Command: checkCommand(t, "pass")},
		{ID: "layout", QualityID: "UI-001", Command: checkCommand(t, "pass")},
		{ID: "input", QualityID: "FE-001", Command: checkCommand(t, "pass")},
	}
	writeVerificationConfig(t, root, checks)
	run(t, 0, "-C", root, "verify", id)
	checks[1].Command = []string{filepath.Join(root, "missing-executable")}
	checks[2].Command = checkCommand(t, "wait")
	writeVerificationConfig(t, root, checks[1:])
	run(t, 1, "-C", root, "verify", "-timeout=500ms", id)
	out, _ := run(t, 0, "-C", root, "report", "-format=text", id)
	if !strings.Contains(out, "合格 0 / 不合格 0 / 未確認 1 / 実行エラー 2") {
		t.Fatal(out)
	}
	out, _ = run(t, 0, "-C", root, "report", "-format=json", id)
	if !strings.Contains(out, "制限時間") {
		t.Fatal(out)
	}

	checks[0].QualityID = "unknown"
	writeVerificationConfig(t, root, checks)
	_, errOut := run(t, 1, "-C", root, "validate")
	if !strings.Contains(errOut, "品質IDが未登録") {
		t.Fatal(errOut)
	}
}

// TestVerificationRejectsArtifactSymlink は成果物フォルダのリンクをたどらず保存を止めることを確認する。
func TestVerificationRejectsArtifactSymlink(t *testing.T) {
	root := example(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "verifies")); err != nil {
		t.Skip(err)
	}
	_, errors := run(t, 1, "-C", root, "verify", "TICKET-001")
	if !strings.Contains(errors, "シンボリックリンク") {
		t.Fatal(errors)
	}
	absent(t, filepath.Join(outside, "TICKET-001.json"))
	absent(t, filepath.Join(outside, ".gitignore"))
}

// TestVerifyChecksOutputBeforeRunning は保存先が使えない場合にチェックが起動しないことを確認する。
func TestVerifyChecksOutputBeforeRunning(t *testing.T) {
	root := example(t)
	write(t, filepath.Join(root, "verifies"), "ディレクトリではない保存先")
	writeVerificationConfig(t, root, []dqd.VerificationCheck{
		{ID: "regression", QualityID: "CM-001", Command: checkCommand(t, "mark")},
	})
	_, stderr := run(t, 1, "-C", root, "verify", "TICKET-001")
	if !strings.Contains(stderr, "検証結果の保存先を準備できません") {
		t.Fatal(stderr)
	}
	absent(t, filepath.Join(root, "ran-unrelated"))
}

// TestVerifyTimeoutStopsDescendants はタイムアウトしたチェックで、その子プロセスも終了することを確認する。
func TestVerifyTimeoutStopsDescendants(t *testing.T) {
	switch runtime.GOOS {
	case "aix", "darwin", "dragonfly", "freebsd", "illumos", "linux", "netbsd", "openbsd", "solaris", "windows":
	default:
		t.Skip("この環境にはプロセスグループの終了処理がありません")
	}
	root := example(t)
	writeVerificationConfig(t, root, []dqd.VerificationCheck{
		{ID: "regression", QualityID: "CM-001", Command: checkCommand(t, "spawn")},
	})
	out, _ := run(t, 1, "-C", root, "verify", "-timeout=1s", "TICKET-001")
	if !strings.Contains(out, "実行エラー 1") {
		t.Fatal(out)
	}
	read(t, filepath.Join(root, "child-started"))
	time.Sleep(time.Second)
	absent(t, filepath.Join(root, "child-finished"))
}

// writeVerificationConfig は検証コマンドの宣言をテスト用プロジェクトに書く。
func writeVerificationConfig(t *testing.T, root string, checks []dqd.VerificationCheck) {
	t.Helper()
	data, err := yaml.Marshal(map[string]any{"schema_version": 1, "checks": checks})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "verification.yml"), string(data))
}

// checkCommand は現在のテストバイナリを外部コマンドとして実行するための引数を返す。
func checkCommand(t *testing.T, mode string) []string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return []string{binary, "-test.run=^TestVerificationCheckProcess$", "--", mode}
}

// TestVerificationCheckProcess は子プロセスとして呼ばれたときだけ外部チェックの結果を再現する。
func TestVerificationCheckProcess(t *testing.T) {
	if len(os.Args) != 4 || os.Args[2] != "--" {
		return
	}
	switch os.Args[3] {
	case "pass":
		if _, err := os.Stat("expected.txt"); err != nil {
			t.Fatal(err)
		}
	case "fail":
		fmt.Fprint(os.Stderr, "<script>ログ</script><img src=https://example.invalid/report>外部ログ", strings.Repeat("a", 32*1024))
		os.Exit(3)
	case "skip":
		os.Exit(77)
	case "wait":
		time.Sleep(5 * time.Second)
	case "mark":
		write(t, "ran-unrelated", "適用対象外のチェックが実行されました")
	case "spawn":
		args := checkCommand(t, "child")
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatal(err)
		}
	case "child":
		write(t, "child-started", "子プロセスが起動しました")
		time.Sleep(1500 * time.Millisecond)
		write(t, "child-finished", "子プロセスが処理を完了しました")
	}
}
