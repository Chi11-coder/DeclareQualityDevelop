//go:build !unix && !windows

package dqd

import (
	"context"
	"os/exec"
)

const nonblockingReadFlag = 0

// runCheckProcess はプロセスグループ管理のない環境でチェックを実行する。
func runCheckProcess(_ context.Context, cmd *exec.Cmd) error {
	return cmd.Run()
}
