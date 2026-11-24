// Package dqd は品質要求の解析、Context生成、検証結果の保存を扱う。
package dqd

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Project はプロジェクト内のファイルにアクセスするためのルートを保持する。
type Project struct {
	root *os.Root
}

const maxProjectInputBytes = 4 << 20

var projectNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ValidateProjectName はプロジェクト名が一つの安全なディレクトリ名として使えることを確認する。
func ValidateProjectName(name string) error {
	if !projectNamePattern.MatchString(name) || len(name) > 255 {
		return errors.New("プロジェクト名は英数字で始まる255文字以内の英数字・ハイフン・アンダースコアで指定してください")
	}
	return nil
}

// OpenProject は指定したディレクトリを、ファイル操作の範囲を制限するプロジェクトルートとして開く。
func OpenProject(path string) (*Project, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("プロジェクトを開けません: %w", err)
	}
	return &Project{root: root}, nil
}

// Close はプロジェクトルートのファイル操作資源を解放する。
func (p *Project) Close() error { return p.root.Close() }

// Read はプロジェクト内の通常ファイルを最大4 MiBまで読み込む。
func (p *Project) Read(path string) ([]byte, error) {
	if !filepath.IsLocal(path) {
		return nil, fmt.Errorf("プロジェクト内の相対パスが必要です: %q", path)
	}
	info, err := p.root.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: 入力には通常ファイルを指定してください", path)
	}
	file, err := p.root.OpenFile(path, os.O_RDONLY|nonblockingReadFlag, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: 入力には通常ファイルを指定してください", path)
	}
	if info.Size() > maxProjectInputBytes {
		return nil, fmt.Errorf("%s: 入力ファイルは4 MiB以下にしてください", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProjectInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(data) > maxProjectInputBytes {
		return nil, fmt.Errorf("%s: 入力ファイルは4 MiB以下にしてください", path)
	}
	return data, nil
}

// TicketIDs はtickets直下のMarkdownファイルから、ファイル名順にチケットIDを取得する。
func (p *Project) TicketIDs() ([]string, error) {
	entries, err := fs.ReadDir(p.root.FS(), "tickets")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tickets: %w", err)
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		if !ValidID(id) {
			return nil, fmt.Errorf("チケットのファイル名が不正です: %s", entry.Name())
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Write は出力先を検証し、一時ファイル経由で内容を保存する。既存ファイルの更新には明示指定を必要とする。
func (p *Project) Write(path string, data []byte, replace bool) error {
	return writeRootFile(p.root, path, data, replace, 0644)
}

// writeRootFile は指定ルート内で、一時ファイルに指定の権限で書き込んでから公開する。
func writeRootFile(root *os.Root, path string, data []byte, replace bool, perm fs.FileMode) error {
	if !filepath.IsLocal(path) || filepath.Clean(path) == "." {
		return fmt.Errorf("出力先にはプロジェクト内のファイル相対パスが必要です: %q", path)
	}
	for current := filepath.Clean(path); current != "."; current = filepath.Dir(current) {
		info, err := root.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("出力先にシンボリックリンクは使用できません: %s", current)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := root.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("出力ディレクトリ: %w", err)
	}
	file, temporary, err := createRootTempFile(root, filepath.Dir(path), perm)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if replace {
		err = root.Rename(temporary, path)
	} else {
		err = root.Link(temporary, path)
	}
	if os.IsExist(err) {
		return fmt.Errorf("%sは既に存在します（上書きする場合は-force）", path)
	}
	if err != nil {
		return fmt.Errorf("%sの保存に失敗しました: %w", path, err)
	}
	return nil
}

// createRootTempFile は指定ルート内に名前が衝突しない一時ファイルを作る。
func createRootTempFile(root *os.Root, dir string, perm fs.FileMode) (*os.File, string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, "", err
	}
	name := filepath.Join(dir, ".dqd-"+hex.EncodeToString(token[:])+".tmp")
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	return file, name, err
}

// InitProject は親ディレクトリ配下に新しい名称フォルダを作り、品質要求とチケットの配置を初期化する。
func InitProject(parent, name string) (string, error) {
	if err := ValidateProjectName(name); err != nil {
		return "", err
	}
	absoluteParent, err := filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	base, err := OpenProject(absoluteParent)
	if err != nil {
		return "", err
	}
	defer base.Close()
	if err := base.root.Mkdir(name, 0755); err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%sは既に存在するため作成しません", filepath.Join(absoluteParent, name))
		}
		return "", fmt.Errorf("プロジェクトディレクトリを作成できません: %w", err)
	}
	child, err := base.root.OpenRoot(name)
	if err != nil {
		base.root.Remove(name)
		return "", err
	}
	p := &Project{root: child}
	complete := false
	var createdFiles, createdDirs []string
	// 初期化が途中で失敗した場合は、今回作成した既知の雛形だけを取り除く。
	defer func() {
		if !complete {
			for i := len(createdFiles) - 1; i >= 0; i-- {
				p.root.Remove(createdFiles[i])
			}
			for i := len(createdDirs) - 1; i >= 0; i-- {
				p.root.Remove(createdDirs[i])
			}
		}
		p.Close()
		if !complete {
			base.root.Remove(name)
		}
	}()
	for _, dir := range []struct {
		name string
		perm fs.FileMode
	}{{"quality", 0755}, {"tickets", 0755}, {"verifies", 0700}, {"reports", 0700}} {
		if err := p.root.Mkdir(dir.name, dir.perm); err != nil {
			return "", err
		}
		createdDirs = append(createdDirs, dir.name)
	}
	for _, file := range []struct{ path, text string }{
		{"requirements.yml", initialRegistry},
		{"quality/common.md", initialCommon},
		{"verification.yml", "schema_version: 1\nchecks: []\n"},
		{filepath.Join("verifies", ".gitignore"), artifactGitIgnore},
		{filepath.Join("reports", ".gitignore"), artifactGitIgnore},
	} {
		if err := p.Write(file.path, []byte(file.text), false); err != nil {
			return "", err
		}
		createdFiles = append(createdFiles, file.path)
	}
	complete = true
	return filepath.Join(absoluteParent, name), nil
}

const initialRegistry = `schema_version: 1

requirements:
  - id: CM-001
    name: 既存機能の維持
    domain: common
    characteristic: functional_suitability
    document:
      path: quality/common.md
      section: CM-001
    applicability:
      mode: always
    verification:
      type: automated
      method: e2e
`

const initialCommon = `# 共通の品質要求

## CM-001：既存機能の維持

### 品質要求

今回の変更によって、既存機能を損なわないこと。

### 共通合格基準

- 変更の影響を受ける既存機能を確認する。
- 既存のテストと今回追加したテストの結果を確認する。
- 未検証の条件を合格として扱わない。

### 検証方法

対象の操作をE2Eテストで確認する。未自動化の項目は、手動確認として区別して記録する。
`
