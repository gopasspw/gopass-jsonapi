//go:build !unix

package jsonapi

import "os/exec"

func detachPinentryTTY(_ *exec.Cmd) {}
