package storage

import (
	"github.com/coreos/go-semver/semver"
	"github.com/juju/errgo/errors"
)

type VersionStorageLocal struct {
	version *semver.Version
}

func (s VersionStorageLocal) ReadVersionFile(file string) (*semver.Version, error) {
	return s.version, nil
}

func (s VersionStorageLocal) WriteVersionFile(file string, version semver.Version) error {
	s.version = &version

	return nil
}

func (s VersionStorageLocal) VersionFileExists(file string) bool {
	return true
}

func NewVersionStorageLocal(versionString string) (*VersionStorageLocal, error) {
	version, err := semver.NewVersion(filterVersionNumber(versionString))

	if err != nil {
		return nil, errors.Mask(err)
	}

	return &VersionStorageLocal{version: version}, nil
}
