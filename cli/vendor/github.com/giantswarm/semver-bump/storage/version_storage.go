package storage

import (
	"github.com/coreos/go-semver/semver"
)

type VersionStorage interface {
	ReadVersionFile(file string) (*semver.Version, error)

	WriteVersionFile(file string, version semver.Version) error

	VersionFileExists(file string) bool
}

func NewVersionStorage(versionStorageType string, localDefaultVersion string) (VersionStorage, error) {
	switch versionStorageType {
	case "local":
		return NewVersionStorageLocal(localDefaultVersion)
	case "file":
		return VersionStorageFile{}, nil
	default:
		panic("Unknown storage backend: " + versionStorageType)
	}
}
