package tests

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	s := SetupDefaultSuite()
	defer Cleanup()
	defer s.Cancel()
	os.Exit(m.Run())
}
