package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"dqd/internal/dqd"
)

// initProject は名前を入力またはフラグから取得し、親ディレクトリ配下に新しいプロジェクトを作成する。
func initProject(fs *flag.FlagSet, args []string, parent *string, in io.Reader, out io.Writer) error {
	usage := "dqd init [-C PARENT] [-name PROJECT_NAME]"
	fs.Lookup("C").Usage = "新しいプロジェクトを作成する親ディレクトリ"
	name := fs.String("name", "", "プロジェクト名（省略時は対話入力）")
	if err := parse(fs, args, usage, out); err != nil {
		return err
	}
	if err := expect(fs, 0, usage); err != nil {
		return err
	}
	if *name == "" {
		value, err := promptProjectName(in, out)
		if err != nil {
			return err
		}
		*name = value
	}
	path, err := dqd.InitProject(*parent, *name)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Initialized: %s\n", path)
	return err
}

// promptProjectName は有効な名前が入力されるまで尋ね、入力終了時は作成せずにエラーを返す。
func promptProjectName(in io.Reader, out io.Writer) (string, error) {
	reader := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprint(out, "プロジェクト名を入力してください: "); err != nil {
			return "", err
		}
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return "", fmt.Errorf("プロジェクト名を読み込めません: %w", readErr)
		}
		name := strings.TrimSpace(line)
		if name == "" && errors.Is(readErr, io.EOF) {
			return "", errors.New("プロジェクト名が入力されませんでした")
		}
		if err := dqd.ValidateProjectName(name); err != nil {
			if errors.Is(readErr, io.EOF) {
				return "", err
			}
			if _, writeErr := fmt.Fprintln(out, err); writeErr != nil {
				return "", writeErr
			}
			continue
		}
		return name, nil
	}
}
