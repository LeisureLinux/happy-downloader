//go:build !windows

package tools

import "os/exec"

// Quiet does nothing here, because there is nothing to do.
//
// On Unix a program that starts a child inherits the terminal it has, and a
// child started with no terminal has none: there is no separate console to
// appear. The equivalent problem does not exist.
func Quiet(cmd *exec.Cmd) {}
