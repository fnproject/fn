// +build !windows

package main

import (
	"io"
	"os"
	"strings"
)

func stdin() io.Reader {
	var stdin io.Reader = os.Stdin
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
		stdin = strings.NewReader("")
	}
	return stdin
}
