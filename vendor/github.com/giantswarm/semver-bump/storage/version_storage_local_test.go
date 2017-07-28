package storage

import (
	"testing"

	"github.com/coreos/go-semver/semver"
)

func TestNewVersionStorageLocal(t *testing.T) {
	versionString := "no-version"
	_, err := NewVersionStorageLocal(versionString)

	if err == nil {
		t.Fatalf("NewVersionStorageLocal: Expected to get an error on wrong version number %s", versionString)
	}

	versionString = "1.0.1"

	_, err = NewVersionStorageLocal(versionString)

	if err != nil {
		t.Fatalf("NewVersionStorageLocal: %s", err)
	}
}

func TestLocalReadVersionFile(t *testing.T) {
	expectedVersion, err := semver.NewVersion("1.1.0")

	if err != nil {
		t.Fatalf("ReadVersionFile: %s", err)
	}

	s := VersionStorageLocal{expectedVersion}
	v, err := s.ReadVersionFile("test")

	if err != nil {
		t.Fatalf("ReadVersionFile: %s", err)
	}

	if expectedVersion.String() != v.String() {
		t.Fatalf("ReadVersionFile: Expected read version to be %s but got %s", expectedVersion.String(), v.String())
	}
}

func TestLocalWriteVersionFile(t *testing.T) {
	expectedVersion, err := semver.NewVersion("1.1.1")

	if err != nil {
		t.Fatalf("WriteVersionFile: %s", err)
	}

	s := VersionStorageLocal{expectedVersion}

	s.WriteVersionFile("testfile", *expectedVersion)
	v, err := s.ReadVersionFile("testfile")

	if err != nil {
		t.Fatalf("WriteVersionFile: %s", v)
	}

	if expectedVersion.String() != v.String() {
		t.Fatalf("WriteVersionFile: Expected written version to be %s but got %s", expectedVersion.String(), v.String())
	}
}

func TestLocalVersionFileExists(t *testing.T) {
	v, err := semver.NewVersion("0.0.0")

	if err != nil {
		t.Fatalf("TestLocalVersionFileExists: %s", err)
	}

	s := VersionStorageLocal{v}

	if s.VersionFileExists("testfile") {
		t.Fatalf("TestLocalVersionFileExists: Expected VersionStorageLocal to pretend no version file exists on version 0.0.0")
	}

	v, err = semver.NewVersion("1.1.1")

	if err != nil {
		t.Fatalf("TestLocalVersionFileExists: %s", err)
	}

	s = VersionStorageLocal{v}

	if !s.VersionFileExists("testfile") {
		t.Fatalf("TestLocalVersionFileExists: Expected VersionStorageLocal to pretend a version file exists on real versions")
	}
}
