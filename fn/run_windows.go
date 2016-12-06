// +build windows

package main

import (
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func stdin() io.Reader {
	var stdin io.Reader = os.Stdin
	if isTerminal(int(os.Stdin.Fd())) {
		stdin = strings.NewReader("")
	}
	return stdin
}

func isTerminal(fd int) bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode := kernel32.NewProc("GetConsoleMode")
	var st uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, uintptr(fd), uintptr(unsafe.Pointer(&st)), 0)
	return r != 0 && e == 0
}
