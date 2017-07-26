// +build !windows

package main

import (
	"io"
	"os"
)

func stdin() io.Reader {
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
		return nil
	}
	return os.Stdin
}
