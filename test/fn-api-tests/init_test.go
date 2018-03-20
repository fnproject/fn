package tests

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	result := m.Run()

	if result == 0 {
		fmt.Fprintln(os.Stdout, "😀  👍  🎗")
	}
	os.Exit(result)
}
