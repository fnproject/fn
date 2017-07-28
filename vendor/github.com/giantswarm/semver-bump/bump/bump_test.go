package bump

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/giantswarm/semver-bump/storage"
)

func NewTestSemverBumper(t *testing.T, initialVersion string) *SemverBumper {
	s, err := storage.NewVersionStorageLocal(initialVersion)

	if err != nil {
		t.Fatalf("NewVersionStorageLocal: %s", err)
	}

	return NewSemverBumper(s, "testfile")
}

func TestBumpMajorVersion(t *testing.T) {
	sb := NewTestSemverBumper(t, "1.0.0")

	v, err := sb.BumpMajorVersion("", "")

	if err != nil {
		t.Fatalf("BumpMajorVersion: %s", err)
	}

	expectedVersion := "2.0.0"

	if expectedVersion != v.String() {
		t.Fatalf("BumpMajorVersion: Expected bumping of major version would result in %s but got %s", expectedVersion, v.String())
	}
}

func TestBumpMinorVersion(t *testing.T) {
	sb := NewTestSemverBumper(t, "1.0.0")

	v, err := sb.BumpMinorVersion("", "")

	if err != nil {
		t.Fatalf("BumpMinorVersion: %s", err)
	}

	expectedVersion := "1.1.0"

	if expectedVersion != v.String() {
		t.Fatalf("BumpMinorVersion: Expected bumping of minor version would result in %s but got %s", expectedVersion, v.String())
	}
}

func TestBumpPatchVersion(t *testing.T) {
	sb := NewTestSemverBumper(t, "1.0.0")

	v, err := sb.BumpPatchVersion("", "")

	if err != nil {
		t.Fatalf("BumpPatchVersion: %s", err)
	}

	expectedVersion := "1.0.1"

	if expectedVersion != v.String() {
		t.Fatalf("BumpPatchVersion: Expected bumping of patch version would result in %s but got %s", expectedVersion, v.String())
	}
}

func TestGetCurrentVersion(t *testing.T) {
	expectedVersion := "2.13.4"
	sb := NewTestSemverBumper(t, expectedVersion)

	v, err := sb.GetCurrentVersion()

	if err != nil {
		t.Fatalf("GetCurrentVersion: %s", err)
	}

	if expectedVersion != v.String() {
		t.Fatalf("GetCurrentVersion: Epexcted to receive version %s but got %s", expectedVersion, v.String())
	}
}

func TestInitVersion(t *testing.T) {
	expectedVersion, err := semver.NewVersion("1.2.45")

	if err != nil {
		t.Fatalf("InitVersion: %s", err)
	}

	sb := NewTestSemverBumper(t, "1.1.0")

	err = sb.InitVersion(*expectedVersion)

	if err == nil {
		t.Fatalf("InitVersion: Expected SemverBumper to return an error when trying to initialize over an existing version")
	}

	sb = NewTestSemverBumper(t, "0.0.0")

	err = sb.InitVersion(*expectedVersion)

	if err != nil {
		t.Fatalf("InitVersion: Expected SemverBumper to initialize new version %s but got error: %s", expectedVersion, err)
	}

	v, err := sb.GetCurrentVersion()

	if err != nil {
		t.Fatalf("InitVersion: %s", err)
	}

	if expectedVersion.String() != v.String() {
		t.Fatalf("InitVersion: Expected SemverBumper to initialize version %s but got %s", expectedVersion.String(), v.String())
	}
}
