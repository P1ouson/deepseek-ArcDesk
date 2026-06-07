//go:build !windows

package main

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func startPTY(cmd *exec.Cmd) (ptyCloser, func() int, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, err
	}
	waitFn := func() int {
		err := cmd.Wait()
		if err == nil {
			return 0
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return &unixPTY{ptmx}, waitFn, nil
}

type unixPTY struct {
	*os.File
}

func (u *unixPTY) resize(cols, rows uint16) error {
	return pty.Setsize(u.File, &pty.Winsize{Rows: rows, Cols: cols})
}
