package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dqd/internal/dqd"
)

// runVerificationCommand はverifyとreportの引数を解析し、検証の実行または保存済み結果の出力を行う。
func runVerificationCommand(command string, fs *flag.FlagSet, args []string, root *string, out io.Writer) error {
	usage := "dqd " + command + " [FLAGS] TICKET_ID"
	var timeout time.Duration
	var format string
	var details bool
	if command == "verify" {
		fs.DurationVar(&timeout, "timeout", 2*time.Minute, "チェック1件あたりの制限時間（例: 30s、5m）")
	} else {
		fs.StringVar(&format, "format", "html", "出力形式（htmlは保存、textとjsonは標準出力）")
		fs.BoolVar(&details, "details", false, "HTMLに実行コマンド・作業ディレクトリ・ログを含める")
	}
	if err := parse(fs, args, usage, out); err != nil {
		return err
	}
	if err := expect(fs, 1, usage); err != nil {
		return err
	}
	if command == "verify" && timeout <= 0 {
		return usageError{"-timeoutには正の時間を指定してください"}
	}
	if command == "report" && format != "html" && format != "text" && format != "json" {
		return usageError{"-formatにはhtml・text・jsonを指定してください"}
	}
	if details && format != "html" {
		return usageError{"-detailsはHTML形式でだけ指定できます"}
	}
	p, err := dqd.OpenProject(*root)
	if err != nil {
		return err
	}
	defer p.Close()
	id := fs.Arg(0)
	if command == "verify" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		result, err := dqd.VerifyTicket(ctx, p, id, timeout)
		if err != nil {
			return err
		}
		path, err := dqd.SaveVerificationResult(p, result)
		if err != nil {
			return err
		}
		if err := printVerificationResult(out, result); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, path); err != nil {
			return err
		}
		if result.Counts().Passed != len(result.Requirements) {
			return errors.New("合格していない品質要求があります。reportで結果を確認してください")
		}
		return nil
	}
	result, err := dqd.ReadVerificationResult(p, id)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		return writeJSON(out, result)
	case "text":
		return printVerificationResult(out, result)
	default:
		path, err := dqd.SaveHTMLReport(p, result, details)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, path)
		return err
	}
}

// printVerificationResult はチケット情報、品質要求ごとの状態、集計を端末に表示する。
func printVerificationResult(out io.Writer, result dqd.VerificationResult) error {
	if _, err := fmt.Fprintf(out, "%s：%s\n", result.TicketID, result.TicketTitle); err != nil {
		return err
	}
	for _, req := range result.Requirements {
		if _, err := fmt.Fprintf(out, "%s  %s  %s\n", req.ID, dqd.StatusLabel(req.Status), req.Name); err != nil {
			return err
		}
	}
	counts := result.Counts()
	_, err := fmt.Fprintf(out, "合格 %d / 不合格 %d / 未確認 %d / 実行エラー %d\n",
		counts.Passed, counts.Failed, counts.Unverified, counts.Errors)
	return err
}
