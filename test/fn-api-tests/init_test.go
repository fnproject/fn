package tests

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	s := SetupDefaultSuite()
	result := m.Run()
	s.Cancel()

	if result == 0 {
		fmt.Fprintln(os.Stdout, "😀  👍  🎗")
	}
	os.Exit(result)
}
