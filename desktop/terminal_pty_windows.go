//go:build windows

package main

import (
	"context"
	"os/exec"
	"strings"

	"github.com/UserExistsError/conpty"
)

func startPTY(cmd *exec.Cmd) (ptyCloser, func() int, error) {
	commandLine := strings.Join(cmd.Args, " ")
	opts := make([]conpty.ConPtyOption, 0, 2)
	if cmd.Dir != "" {
		opts = append(opts, conpty.ConPtyWorkDir(cmd.Dir))
	}
	if len(cmd.Env) > 0 {
		opts = append(opts, conpty.ConPtyEnv(cmd.Env))
	}
	cpty, err := conpty.Start(commandLine, opts...)
	if err != nil {
		return nil, nil, err
	}
	waitFn := func() int {
		code, err := cpty.Wait(context.Background())
		if err != nil {
			return 1
		}
		return int(code)
	}
	return &winPTY{cpty}, waitFn, nil
}

type winPTY struct {
	*conpty.ConPty
}

func (w *winPTY) resize(cols, rows uint16) error {
	return w.ConPty.Resize(int(cols), int(rows))
}
