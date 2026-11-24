package dqd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	StatusPassed     = "passed"
	StatusFailed     = "failed"
	StatusUnverified = "unverified"
	StatusError      = "error"

	maxCheckOutput = 32 * 1024
)

// VerificationCheck は一つの品質要求を確認する外部コマンドを表す。
type VerificationCheck struct {
	ID        string   `yaml:"id" json:"id"`
	QualityID string   `yaml:"quality_id" json:"quality_id"`
	Command   []string `yaml:"command" json:"command"`
	Directory string   `yaml:"directory,omitempty" json:"directory,omitempty"`
}

// CheckResult は一つのチェックの実行結果とログを保持する。
type CheckResult struct {
	VerificationCheck
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Message    string `json:"message,omitempty"`
	Output     string `json:"output,omitempty"`
	Truncated  bool   `json:"output_truncated,omitempty"`
}

// RequirementResult は一つの品質要求の判定とチェック結果を保持する。
type RequirementResult struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Method string        `json:"method"`
	Status string        `json:"status"`
	Checks []CheckResult `json:"checks"`
}

// VerificationResult はチケットについて保存する一回分の検証結果を表す。
type VerificationResult struct {
	SchemaVersion int                 `json:"schema_version"`
	TicketID      string              `json:"ticket_id"`
	TicketTitle   string              `json:"ticket_title"`
	StartedAt     time.Time           `json:"started_at"`
	Requirements  []RequirementResult `json:"requirements"`
}

// loadVerificationChecks は任意のverification.ymlを読み、コマンドと品質IDの対応を確認する。
func loadVerificationChecks(p *Project, registry Registry) ([]VerificationCheck, error) {
	data, err := p.Read("verification.yml")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var config struct {
		SchemaVersion int                 `yaml:"schema_version"`
		Checks        []VerificationCheck `yaml:"checks"`
	}
	if err := decodeDeclaration(data, &config); err != nil {
		return nil, fmt.Errorf("verification.yml: %w", err)
	}
	if config.SchemaVersion != 1 {
		return nil, errors.New("verification.yml: schema_versionは1だけに対応します")
	}
	known := make(map[string]bool)
	for _, req := range registry.Requirements {
		known[req.ID] = true
	}
	seen := make(map[string]bool)
	for _, check := range config.Checks {
		if !ValidID(check.ID) || seen[check.ID] {
			return nil, fmt.Errorf("verification.yml: チェックIDが不正または重複しています: %q", check.ID)
		}
		seen[check.ID] = true
		if !known[check.QualityID] {
			return nil, fmt.Errorf("verification.yml: %s: 品質IDが未登録です: %q", check.ID, check.QualityID)
		}
		if len(check.Command) == 0 || strings.TrimSpace(check.Command[0]) == "" {
			return nil, fmt.Errorf("verification.yml: %s: commandの先頭に実行ファイルを指定してください", check.ID)
		}
		for _, arg := range check.Command {
			if strings.ContainsRune(arg, 0) {
				return nil, fmt.Errorf("verification.yml: %s: commandにNUL文字は使えません", check.ID)
			}
		}
		if check.Directory != "" && !filepath.IsLocal(check.Directory) {
			return nil, fmt.Errorf("verification.yml: %s: directoryはプロジェクト内の相対パスにしてください", check.ID)
		}
	}
	return config.Checks, nil
}

// VerifyTicket は要求を選び、対応するチェックだけを順に実行して品質IDごとの結果を返す。
func VerifyTicket(ctx context.Context, p *Project, id string, timeout time.Duration) (VerificationResult, error) {
	var result VerificationResult
	catalog, err := LoadCatalog(p)
	if err != nil {
		return result, err
	}
	ticket, err := LoadTicket(p, id)
	if err != nil {
		return result, err
	}
	resolved, _, err := ResolveTicket(catalog, ticket)
	if err != nil {
		return result, err
	}
	checks, err := loadVerificationChecks(p, catalog.Registry)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := prepareVerificationOutput(p, id); err != nil {
		return result, fmt.Errorf("検証結果の保存先を準備できません: %w", err)
	}
	byQuality := make(map[string][]VerificationCheck)
	for _, check := range checks {
		byQuality[check.QualityID] = append(byQuality[check.QualityID], check)
	}
	result = VerificationResult{
		SchemaVersion: 1,
		TicketID:      ticket.ID,
		TicketTitle:   ticket.Title,
		StartedAt:     time.Now().UTC(),
	}
	for _, item := range resolved {
		req := item.Requirement
		row := RequirementResult{ID: req.ID, Name: req.Name, Method: req.Verification.Method, Checks: []CheckResult{}}
		for _, check := range byQuality[req.ID] {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			row.Checks = append(row.Checks, runVerificationCheck(ctx, p, check, timeout))
		}
		row.Status = requirementStatus(row.Checks)
		result.Requirements = append(result.Requirements, row)
	}
	return result, ctx.Err()
}

// requirementStatus は実行エラー、不合格、未確認の順にまとめ、全チェックが合格した場合だけ合格を返す。
func requirementStatus(checks []CheckResult) string {
	if len(checks) == 0 {
		return StatusUnverified
	}
	status := StatusPassed
	for _, check := range checks {
		switch check.Status {
		case StatusError:
			return StatusError
		case StatusFailed:
			status = StatusFailed
		case StatusUnverified:
			if status == StatusPassed {
				status = StatusUnverified
			}
		}
	}
	return status
}

// runVerificationCheck は作業ディレクトリを確認し、終了コード・タイムアウト・出力を記録する。
func runVerificationCheck(ctx context.Context, p *Project, check VerificationCheck, timeout time.Duration) CheckResult {
	result := CheckResult{VerificationCheck: check, Status: StatusError}
	dir := check.Directory
	if dir == "" {
		dir = "."
	}
	info, err := p.root.Stat(dir)
	if err != nil {
		result.Message = fmt.Sprintf("作業ディレクトリを開けません: %v", err)
		return result
	}
	if !info.IsDir() {
		result.Message = fmt.Sprintf("作業ディレクトリではありません: %s", dir)
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, check.Command[0], check.Command[1:]...)
	cmd.Dir = filepath.Join(p.root.Name(), dir)
	cmd.WaitDelay = time.Second
	var output checkOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	start := time.Now()
	err = runCheckProcess(ctx, cmd)
	result.DurationMS = time.Since(start).Milliseconds()
	result.Output = output.text.String()
	result.Truncated = output.truncated
	if err == nil {
		result.Status = StatusPassed
		return result
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Message = fmt.Sprintf("制限時間（%s）を超えました", timeout)
		return result
	}
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() < 0 {
		result.Message = err.Error()
		return result
	}
	if code := exit.ExitCode(); code == 77 {
		result.Status = StatusUnverified
		result.Message = "チェックがスキップされました（終了コード 77）"
	} else {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("終了コード %d", code)
	}
	return result
}

type checkOutput struct {
	text      strings.Builder
	truncated bool
}

// Write はログの先頭32 KiBを保持し、それ以降はコマンドの実行を止めずに読み捨てる。
func (o *checkOutput) Write(data []byte) (int, error) {
	n := len(data)
	if remaining := maxCheckOutput - o.text.Len(); n > remaining {
		data = data[:remaining]
		o.truncated = true
	}
	o.text.Write(data)
	return n, nil
}
