package main

import (
	"os"

	"dqd/internal/cli"
)

// main は引数と標準入出力をCLIへ渡し、処理結果を終了コードとして返す。
func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
