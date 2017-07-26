// +build windows

package main

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

func stdin() io.Reader {
	if isTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	return os.Stdin
}

func isTerminal(fd int) bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode := kernel32.NewProc("GetConsoleMode")
	var st uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, uintptr(fd), uintptr(unsafe.Pointer(&st)), 0)
	return r != 0 && e == 0
}
