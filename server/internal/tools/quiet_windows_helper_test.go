//go:build windows

package tools

import "syscall"

func newSysProcAttr(flags uint32) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: flags}
}

func newGroupFlag() uint32 {
	return syscall.CREATE_NEW_PROCESS_GROUP
}
