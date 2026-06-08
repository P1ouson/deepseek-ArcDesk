package main

import (
	"os/exec"

	"arcdesk/internal/proc"
)

func setCloudflaredCmdAttrs(cmd *exec.Cmd) {
	proc.HideWindowDetached(cmd)
}
