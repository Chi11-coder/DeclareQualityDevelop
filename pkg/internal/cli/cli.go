// Package cli はDQDのコマンド解析と入出力を扱う。
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"dqd/internal/dqd"

	"gopkg.in/yaml.v3"
)

type usageError struct{ message string }

// Error は引数の誤りを表すエラーの説明を返す。
func (e usageError) Error() string { return e.message }

// Run は標準入力を受け取ってCLIを実行し、成功・宣言エラー・引数エラーに対応する終了コードを返す。
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	err := execute(args, stdin, stdout)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
	var usage usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

const commands = `Commands:
  init                  名前を尋ね、新しいプロジェクトを作成
  validate              宣言と文書・チケットの整合性を検証
  resolve TICKET_ID     適用する品質要求を表示
  context TICKET_ID     開発用Contextを生成
  verify TICKET_ID      品質要求に対応するチェックを実行
  report TICKET_ID      保存した検証結果からHTMLを生成
  quality list          品質要求を一覧表示
  quality show ID       品質要求の本文を表示
  ticket new TICKET_ID  チケットの雛形を作成

Flagsは位置引数より前に指定してください。各コマンドの-hで詳細を表示します。
`

// newFlagSet は標準のflagパッケージで、プロジェクト指定を持つサブコマンドを作成する。
func newFlagSet(name string, root *string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	// ヘルプと引数エラーを呼び出し元で表示するため、FlagSetの自動表示を抑止する。
	fs.Usage = func() {}
	fs.StringVar(root, "C", *root, "プロジェクトルート（requirements.ymlのあるディレクトリ）")
	return fs
}

// parse はフラグを解析し、ヘルプの表示または引数エラーの変換を行う。
func parse(fs *flag.FlagSet, args []string, usage string, stdout io.Writer) error {
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		printHelp(fs, usage, stdout)
		return flag.ErrHelp
	}
	if err != nil {
		return usageError{err.Error() + "（詳細は-h）"}
	}
	return nil
}

// printHelp は使用方法と利用可能なフラグを出力する。
func printHelp(fs *flag.FlagSet, usage string, out io.Writer) {
	fmt.Fprintf(out, "Usage: %s\n\n", usage)
	fs.SetOutput(out)
	fs.PrintDefaults()
	if fs.Name() == "dqd" {
		fmt.Fprint(out, "\n"+commands)
	}
}

// expect は位置引数の数がコマンドの仕様と一致するか確認する。
func expect(fs *flag.FlagSet, count int, usage string) error {
	if fs.NArg() != count {
		return usageError{"Usage: " + usage + "（flagsは位置引数より前に指定してください）"}
	}
	return nil
}

// execute はコマンド名を解析し、対応する初期化・検証・生成処理へ振り分ける。
func execute(args []string, in io.Reader, out io.Writer) error {
	root := "."
	global := newFlagSet("dqd", &root)
	if err := parse(global, args, "dqd [-C DIR] COMMAND [FLAGS] [ARGS]", out); err != nil {
		return err
	}
	if global.NArg() == 0 {
		printHelp(global, "dqd [-C DIR] COMMAND [FLAGS] [ARGS]", out)
		return nil
	}
	command, rest := global.Arg(0), global.Args()[1:]
	if command == "quality" || command == "ticket" {
		if len(rest) == 0 || rest[0] == "-h" || rest[0] == "--help" {
			if command == "quality" {
				fmt.Fprintln(out, "Usage: dqd quality list [FLAGS] | dqd quality show [FLAGS] ID")
			} else {
				fmt.Fprintln(out, "Usage: dqd ticket new [FLAGS] TICKET_ID")
			}
			return nil
		}
		command += " " + rest[0]
		rest = rest[1:]
	}
	fs := newFlagSet("dqd "+command, &root)
	switch command {
	case "init":
		return initProject(fs, rest, &root, in, out)
	case "validate":
		usage := "dqd " + command + " [-C DIR]"
		if err := parse(fs, rest, usage, out); err != nil {
			return err
		}
		if err := expect(fs, 0, usage); err != nil {
			return err
		}
		p, err := dqd.OpenProject(root)
		if err != nil {
			return err
		}
		defer p.Close()
		report, err := dqd.ValidateProject(p)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "Valid: %d requirements, %d tickets\n", report.Requirements, report.Tickets)
		return err
	case "resolve", "context":
		return runTicketCommand(command, fs, rest, &root, out)
	case "quality list", "quality show":
		return runQuality(command, fs, rest, &root, out)
	case "ticket new":
		return newTicket(fs, rest, &root, out)
	case "verify", "report":
		return runVerificationCommand(command, fs, rest, &root, out)
	default:
		return usageError{fmt.Sprintf("不明なコマンドです: %s（dqd -hで一覧を表示）", command)}
	}
}

// runTicketCommand は対象チケットの要求を解決し、一覧表示またはContextの生成を行う。
func runTicketCommand(command string, fs *flag.FlagSet, args []string, root *string, out io.Writer) error {
	usage := "dqd " + command + " [FLAGS] TICKET_ID"
	var jsonOutput, toStdout, force bool
	if command == "resolve" {
		fs.BoolVar(&jsonOutput, "json", false, "JSON形式で出力")
	} else {
		fs.BoolVar(&toStdout, "stdout", false, "ファイルを作成せずContextを標準出力へ表示")
		fs.BoolVar(&force, "force", false, "既存のContextを上書き")
	}
	if err := parse(fs, args, usage, out); err != nil {
		return err
	}
	if err := expect(fs, 1, usage); err != nil {
		return err
	}
	p, err := dqd.OpenProject(*root)
	if err != nil {
		return err
	}
	defer p.Close()
	catalog, err := dqd.LoadCatalog(p)
	if err != nil {
		return err
	}
	ticket, err := dqd.LoadTicket(p, fs.Arg(0))
	if err != nil {
		return err
	}
	resolved, text, err := dqd.ResolveTicket(catalog, ticket)
	if err != nil {
		return err
	}
	if command == "resolve" {
		if jsonOutput {
			return writeJSON(out, resolved)
		}
		writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "ID\tNAME\tREASON")
		for _, item := range resolved {
			fmt.Fprintf(writer, "%s\t%s\t%s\n", item.Requirement.ID, item.Requirement.Name, item.Reason)
		}
		return writer.Flush()
	}
	if toStdout {
		_, err := io.WriteString(out, text)
		return err
	}
	path := filepath.Join(".dqd", "contexts", ticket.ID+".md")
	if err := p.Write(path, []byte(text), force); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, filepath.ToSlash(path))
	return err
}

// runQuality は品質要求の一覧、JSON、または指定した要求の本文を表示する。
func runQuality(command string, fs *flag.FlagSet, args []string, root *string, out io.Writer) error {
	usage, count := "dqd quality list [FLAGS]", 0
	var jsonOutput bool
	if command == "quality list" {
		fs.BoolVar(&jsonOutput, "json", false, "JSON形式で出力")
	} else {
		usage, count = "dqd quality show [FLAGS] ID", 1
	}
	if err := parse(fs, args, usage, out); err != nil {
		return err
	}
	if err := expect(fs, count, usage); err != nil {
		return err
	}
	p, err := dqd.OpenProject(*root)
	if err != nil {
		return err
	}
	defer p.Close()
	catalog, err := dqd.LoadCatalog(p)
	if err != nil {
		return err
	}
	if command == "quality show" {
		if text, ok := catalog.Details[fs.Arg(0)]; ok {
			_, err := fmt.Fprintln(out, text)
			return err
		}
		return fmt.Errorf("品質IDが未登録です: %s", fs.Arg(0))
	}
	if jsonOutput {
		return writeJSON(out, catalog.Registry.Requirements)
	}
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "ID\tNAME\tDOMAIN\tMODE")
	for _, req := range catalog.Registry.Requirements {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", req.ID, req.Name, req.Domain, req.Applicability.Mode)
	}
	return writer.Flush()
}

// newTicket は指定したタイトル・領域・品質IDからチケット雛形を検証して作成する。
func newTicket(fs *flag.FlagSet, args []string, root *string, out io.Writer) error {
	usage := "dqd ticket new [FLAGS] TICKET_ID"
	title := fs.String("title", "実装する機能の名前", "チケットのタイトル")
	domains := fs.String("domains", "", "変更対象の技術領域（カンマ区切り）")
	include := fs.String("include", "", "明示追加する品質ID（カンマ区切り）")
	if err := parse(fs, args, usage, out); err != nil {
		return err
	}
	if err := expect(fs, 1, usage); err != nil {
		return err
	}
	id := fs.Arg(0)
	if !dqd.ValidID(id) {
		return fmt.Errorf("チケットIDが不正です: %q", id)
	}
	if err := dqd.ValidateText(*title, "title"); err != nil {
		return err
	}
	metadata := dqd.TicketMetadata{ID: id, Title: *title, Domains: csv(*domains), Attributes: []string{}}
	metadata.Quality.Include = csv(*include)
	yamlData, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}
	content := "---\n" + string(yamlData) + "---\n\n# 機能要件\n\n実装する機能、操作、期待する結果を記述してください。\n\n# 個別合格条件\n"
	p, err := dqd.OpenProject(*root)
	if err != nil {
		return err
	}
	defer p.Close()
	catalog, err := dqd.LoadCatalog(p)
	if err != nil {
		return err
	}
	ticket, err := dqd.ParseTicket([]byte(content), id)
	if err != nil {
		return err
	}
	if _, _, err := dqd.ResolveTicket(catalog, ticket); err != nil {
		return err
	}
	path := filepath.Join("tickets", id+".md")
	if err := p.Write(path, []byte(content), false); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, filepath.ToSlash(path))
	return err
}

// csv はカンマ区切りの文字列を分割し、空白と空要素を取り除く。
func csv(value string) []string {
	result := []string{}
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

// writeJSON は値をインデント付きのJSONとして出力する。
func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
