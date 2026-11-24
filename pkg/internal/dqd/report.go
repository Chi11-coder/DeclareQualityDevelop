package dqd

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"strings"
)

// VerificationCounts は判定ごとの品質要求の件数を表す。
type VerificationCounts struct {
	Passed     int
	Failed     int
	Unverified int
	Errors     int
}

// Counts は品質要求の合格、不合格、未確認、実行エラーを数える。
func (r VerificationResult) Counts() VerificationCounts {
	var counts VerificationCounts
	for _, req := range r.Requirements {
		switch req.Status {
		case StatusPassed:
			counts.Passed++
		case StatusFailed:
			counts.Failed++
		case StatusError:
			counts.Errors++
		default:
			counts.Unverified++
		}
	}
	return counts
}

// SaveVerificationResult は直近の検証結果を、チケットごとのJSONとして保存する。
func SaveVerificationResult(p *Project, result VerificationResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return saveReportFile(p, "verifies", result.TicketID, ".json", append(data, '\n'))
}

// ReadVerificationResult は指定チケットの保存済み結果を読み込み、形式とチケットIDを確認する。
func ReadVerificationResult(p *Project, id string) (VerificationResult, error) {
	var result VerificationResult
	if !ValidID(id) {
		return result, fmt.Errorf("チケットIDが不正です: %q", id)
	}
	name := id + ".json"
	data, path, err := readProjectVerification(p, name)
	if errors.Is(err, os.ErrNotExist) {
		data, path, err = readCachedReportFile(p, name, currentReportLayout)
	}
	if errors.Is(err, os.ErrNotExist) {
		data, path, err = readCachedReportFile(p, name, legacyReportLayout)
	}
	if errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("%sの検証結果がありません。この端末で先にverifyを実行してください", id)
	}
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("%s: %w", path, err)
	}
	if result.SchemaVersion != 1 || result.TicketID != id || result.StartedAt.IsZero() || len(result.Requirements) == 0 {
		return result, fmt.Errorf("%s: 保存された検証結果の形式またはチケットIDが不正です", path)
	}
	return result, nil
}

// readProjectVerification はプロジェクト内に保存したチケットの検証結果を読む。
func readProjectVerification(p *Project, name string) ([]byte, string, error) {
	dir, err := openArtifactDirectory(p, "verifies", false)
	if err != nil {
		return nil, "", err
	}
	defer dir.Close()
	return readPrivateReportFile(dir, name)
}

//go:embed report.html
var reportHTML string

//go:embed report.css
var reportCSS string

var reportTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"statusLabel": StatusLabel,
}).Parse(reportHTML))

// StatusLabel は検証結果の状態を日本語の表示名に変換する。
func StatusLabel(status string) string {
	switch status {
	case StatusPassed:
		return "合格"
	case StatusFailed:
		return "不合格"
	case StatusError:
		return "実行エラー"
	default:
		return "未確認"
	}
}

// SaveHTMLReport は外部リソースを使わないHTMLを作り、指定された場合はコマンドとログも含める。
func SaveHTMLReport(p *Project, result VerificationResult, details bool) (string, error) {
	var output bytes.Buffer
	// HTMLの改行の扱いに合わせ、埋め込む固定CSSとCSPのハッシュを同じ内容から作る。
	style := strings.ReplaceAll(reportCSS, "\r\n", "\n")
	hash := sha256.Sum256([]byte(style))
	data := struct {
		Result    VerificationResult
		Counts    VerificationCounts
		Details   bool
		Style     template.CSS
		StyleHash string
	}{
		Result:    result,
		Counts:    result.Counts(),
		Details:   details,
		Style:     template.CSS(style),
		StyleHash: base64.StdEncoding.EncodeToString(hash[:]),
	}
	if err := reportTemplate.Execute(&output, data); err != nil {
		return "", err
	}
	return saveReportFile(p, "reports", result.TicketID, ".html", output.Bytes())
}
