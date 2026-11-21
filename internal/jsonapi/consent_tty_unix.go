//go:build unix

package jsonapi

import (
	"os/exec"
	"syscall"
)

func detachPinentryTTY(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
