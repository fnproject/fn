package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/go-semver/semver"
)

func TestFileReadVersionFile(t *testing.T) {
	filename := "not-existing"
	s := &VersionStorageFile{}

	_, err := s.ReadVersionFile(filename)

	if err == nil {
		t.Fatalf("ReadVersionFile %s: Expected error because of not existing version file, none received", filename)
	}

	filename, err = createTempVersionFile("no-version")

	if err != nil {
		t.Fatalf("ReadVersionFile: %s", err)
	}

	defer os.Remove(filename)

	_, err = s.ReadVersionFile(filename)

	if err == nil {
		t.Fatalf("ReadVersionFile %s: Expected error because of invalid version format, none received", filename)
	}

	filename, err = createTempVersionFile("1.0.0\n ")

	if err != nil {
		t.Fatalf("ReadVersionFile: %s", err)
	}

	defer os.Remove(filename)

	v, err := s.ReadVersionFile(filename)

	if err != nil {
		t.Fatalf("ReadVersionFile %s: Could not read version file. Got error: %s", filename, err)
	}

	if "1.0.0" != v.String() {
		t.Fatalf("ReadVersionFile %s: Version does not match. Expected %s, got %s", "1.0.0", v.String())
	}
}

func TestFileWriteVersionFile(t *testing.T) {
	filename, err := createTempVersionFile("1.0.0")

	if err != nil {
		t.Fatalf("WriteVersionFile: %s", err)
	}

	defer os.Remove(filename)

	s := &VersionStorageFile{}
	v, err := semver.NewVersion("1.1.1")

	if err != nil {
		t.Fatalf("WriteVersionFile: %s", err)
	}

	err = s.WriteVersionFile(filename, *v)

	if err != nil {
		t.Fatalf("WriteVersionFile: %s", err)
	}

	readVersion, err := s.ReadVersionFile(filename)

	if err != nil {
		t.Fatalf("WriteVersionFile: %s", err)
	}

	if v.String() != readVersion.String() {
		t.Fatalf("WriteVersionFile: Expected that version %s would be written but got %s", v.String(), readVersion.String())
	}
}

func TestFileVersionFileExists(t *testing.T) {
	filename := "not-existing"
	s := &VersionStorageFile{}

	if s.VersionFileExists(filename) {
		t.Fatalf("VersionFileExists: Expected version file %s to not exist", filename)
	}

	filename, err := createTempVersionFile("1.0.0")

	if err != nil {
		t.Fatalf("VersionFileExists: %s", err)
	}

	defer os.Remove(filename)

	if !s.VersionFileExists(filename) {
		t.Fatalf("VersionFileExists: Expected file %s to exist", filename)
	}
}

func createTempVersionFile(version string) (string, error) {
	file, err := ioutil.TempFile("", "")

	if err != nil {
		return "", fmt.Errorf("Cannot create temporary version file ")
	}

	err = ioutil.WriteFile(file.Name(), []byte(version), 0664)

	if err != nil {
		return "", fmt.Errorf("Cannot write to temporary veersion file")
	}

	return file.Name(), nil
}
