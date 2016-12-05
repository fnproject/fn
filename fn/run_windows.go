// +build windows

package main

import (
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func getStdin() io.Reader {
	var stdin io.Reader = os.Stdin
	if isTerminal(int(os.Stdin.Fd())) {
		stdin = strings.NewReader("")
	}
	return stdin
}

func isTerminal(fd int) bool {
	var st uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, uintptr(fd), uintptr(unsafe.Pointer(&st)), 0)
	return r != 0 && e == 0
}
