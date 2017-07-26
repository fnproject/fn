package storage

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/juju/errgo/errors"
)

type VersionStorageFile struct{}

func (s VersionStorageFile) ReadVersionFile(file string) (*semver.Version, error) {
	versionBuffer, err := ioutil.ReadFile(file)

	if err != nil {
		return nil, errors.Mask(err)
	}

	versionString := string(versionBuffer)
	versionString = strings.TrimSpace(versionString)
	versionString = filterVersionNumber(versionString)

	version, err := semver.NewVersion(versionString)

	if err != nil {
		return nil, errors.Mask(err)
	}

	return version, nil
}

func (s VersionStorageFile) WriteVersionFile(file string, version semver.Version) error {
	return errors.Mask(ioutil.WriteFile(file, []byte(version.String()), 0664))
}

func (s VersionStorageFile) VersionFileExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}

	return false
}
