package dqd

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const artifactGitIgnore = "*\n!.gitignore\n"

type reportLayout uint8

const (
	currentReportLayout reportLayout = iota
	legacyReportLayout
)

// openCachedReportDirectory はプロジェクトに対応するキャッシュのディレクトリを開く。
func openCachedReportDirectory(p *Project, layout reportLayout) (*os.Root, error) {
	project, err := filepath.Abs(p.root.Name())
	if err != nil {
		return nil, err
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return nil, err
	}
	cache := os.Getenv("DQD_CACHE_DIR")
	if cache == "" {
		cache, err = os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("利用者のキャッシュ先を取得できません: %w", err)
		}
	}
	if !filepath.IsAbs(cache) {
		return nil, fmt.Errorf("キャッシュ先には絶対パスが必要です: %q", cache)
	}
	cache = filepath.Clean(cache)
	if err := checkReportLocation(cache, project); err != nil {
		return nil, err
	}
	cache, err = filepath.EvalSymlinks(cache)
	if err != nil {
		return nil, err
	}
	if err := checkReportLocation(cache, project); err != nil {
		return nil, err
	}
	base, err := os.OpenRoot(cache)
	if err != nil {
		return nil, err
	}
	defer base.Close()

	key := fmt.Sprintf("%x", sha256.Sum256([]byte(project)))
	parts := []string{"dqd", key, "reports"}
	if layout == legacyReportLayout {
		parts = []string{"dqd", "reports", key}
	}
	if err := checkReportLocation(filepath.Join(cache, filepath.Join(parts...)), project); err != nil {
		return nil, err
	}
	info, err := base.Stat(".")
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0022 != 0 {
		return nil, fmt.Errorf("キャッシュ先に他の利用者への書き込み権限があります: %s", cache)
	}
	dir := ""
	for _, part := range parts {
		dir = filepath.Join(dir, part)
		info, err := base.Lstat(dir)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || !privateReportMode(info.Mode()) {
			return nil, fmt.Errorf("レポートの保存先には利用者専用のディレクトリが必要です: %s", filepath.Join(cache, dir))
		}
	}
	return base.OpenRoot(dir)
}

// checkReportLocation は対象プロジェクトやGit作業ツリーの中への保存を拒否する。
func checkReportLocation(cache, project string) error {
	if rel, err := filepath.Rel(project, cache); err == nil && filepath.IsLocal(rel) {
		return fmt.Errorf("キャッシュ先を対象プロジェクトの外に指定してください: %s", cache)
	}
	for dir := cache; ; dir = filepath.Dir(dir) {
		git := filepath.Join(dir, ".git")
		info, err := os.Lstat(git)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil {
			// worktreeの.gitファイルと、HEADを持つ.gitディレクトリを判別する。
			if info.IsDir() {
				_, err = os.Stat(filepath.Join(git, "HEAD"))
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			if err == nil {
				return fmt.Errorf("キャッシュ先をGit作業ツリーの外に指定してください: %s", cache)
			}
		}
		if filepath.Dir(dir) == dir {
			return nil
		}
	}
}

// privateReportMode は他の利用者へのアクセス権がないかを確認する。Windowsでは親ディレクトリのACLに従う。
func privateReportMode(mode os.FileMode) bool {
	return runtime.GOOS == "windows" || mode.Perm()&0077 == 0
}

// readCachedReportFile は利用者キャッシュから検証結果を読み込む。
func readCachedReportFile(p *Project, name string, layout reportLayout) ([]byte, string, error) {
	dir, err := openCachedReportDirectory(p, layout)
	if err != nil {
		return nil, "", err
	}
	defer dir.Close()
	return readPrivateReportFile(dir, name)
}

// readPrivateReportFile は利用者専用の通常ファイルを読み込む。
func readPrivateReportFile(dir *os.Root, name string) ([]byte, string, error) {
	path := filepath.Join(dir.Name(), name)
	info, err := dir.Lstat(name)
	if err != nil {
		return nil, path, err
	}
	if !info.Mode().IsRegular() || !privateReportMode(info.Mode()) {
		return nil, path, fmt.Errorf("検証結果には利用者専用の通常ファイルが必要です: %s", path)
	}
	data, err := dir.ReadFile(name)
	return data, path, err
}

// openArtifactDirectory は利用者が作業するプロジェクトの成果物フォルダを開く。
func openArtifactDirectory(p *Project, folder string, create bool) (*os.Root, error) {
	if create {
		if err := p.root.Mkdir(folder, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	info, err := p.root.Lstat(folder)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0022 != 0) {
		return nil, fmt.Errorf("%sにはシンボリックリンクでなく、他の利用者が書き込めないディレクトリを指定してください", folder)
	}
	return p.root.OpenRoot(folder)
}

// ensureArtifactGitIgnore は成果物を除外し、利用者が既に書いた除外規則も残す。
func ensureArtifactGitIgnore(dir *os.Root) error {
	const name = ".gitignore"
	info, err := dir.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		if writeErr := writeRootFile(dir, name, []byte(artifactGitIgnore), false, 0644); writeErr == nil {
			return nil
		} else if info, err = dir.Lstat(name); err != nil {
			return writeErr
		}
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("通常ファイルの.gitignoreが必要です: %s", dir.Name())
	}
	data, err := dir.ReadFile(name)
	if err != nil {
		return err
	}
	lines := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if strings.HasSuffix("\n"+lines+"\n", "\n"+artifactGitIgnore) {
		return nil
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return writeRootFile(dir, name, append(data, artifactGitIgnore...), true, info.Mode().Perm())
}

// saveReportFile は検証結果を利用者専用の権限で保存し、端末で開ける絶対パスを返す。
func saveReportFile(p *Project, folder, id, extension string, data []byte) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("チケットIDが不正です: %q", id)
	}
	dir, err := openArtifactDirectory(p, folder, true)
	if err != nil {
		return "", err
	}
	defer dir.Close()
	name := id + extension
	if err := checkReportDestination(dir, name); err != nil {
		return "", err
	}
	if err := writeRootFile(dir, name, data, true, 0600); err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(dir.Name(), name))
}

// checkReportDestination は出力ファイルとそのGit除外設定を確認する。
func checkReportDestination(dir *os.Root, name string) error {
	info, err := dir.Lstat(name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && (!info.Mode().IsRegular() || !privateReportMode(info.Mode())) {
		return fmt.Errorf("利用者専用の通常ファイルだけを上書きできます: %s", filepath.Join(dir.Name(), name))
	}
	return ensureArtifactGitIgnore(dir)
}

// prepareVerificationOutput は実行前に検証結果を保存できることを確認する。
func prepareVerificationOutput(p *Project, id string) error {
	dir, err := openArtifactDirectory(p, "verifies", true)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := checkReportDestination(dir, id+".json"); err != nil {
		return err
	}
	file, name, err := createRootTempFile(dir, ".", 0600)
	if err != nil {
		return err
	}
	return errors.Join(file.Close(), dir.Remove(name))
}
