//go:build unix

package dqd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
)

const nonblockingReadFlag = syscall.O_NONBLOCK

// runCheckProcess は独立したプロセスグループでチェックを実行し、終了時にグループを片付ける。
func runCheckProcess(_ context.Context, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return killCheckGroup(cmd.Process.Pid)
	}
	runErr := cmd.Run()
	if cmd.Process == nil {
		return runErr
	}
	cleanupErr := killCheckGroup(cmd.Process.Pid)
	if errors.Is(cleanupErr, os.ErrProcessDone) {
		cleanupErr = nil
	}
	return errors.Join(runErr, cleanupErr)
}

// killCheckGroup はチェック用グループに属するプロセスを終了する。
func killCheckGroup(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGKILL); errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	} else {
		return err
	}
}
