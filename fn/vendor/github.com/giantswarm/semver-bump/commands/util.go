package commands

import (
	"github.com/giantswarm/semver-bump/bump"
	"github.com/giantswarm/semver-bump/storage"
	"github.com/juju/errgo/errors"
)

func getSemverBumper() (*bump.SemverBumper, error) {
	s, err := storage.NewVersionStorage(versionStorageType, versionStorageLocalDefaultVersion)

	if err != nil {
		return nil, errors.Mask(err)
	}

	return bump.NewSemverBumper(s, versionFile), nil
}
