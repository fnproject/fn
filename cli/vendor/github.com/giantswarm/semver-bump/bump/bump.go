package bump

import (
	"github.com/coreos/go-semver/semver"
	"github.com/giantswarm/semver-bump/storage"
	"github.com/juju/errgo/errors"
)

type versionBumpCallback func(version *semver.Version)

type SemverBumper struct {
	storage     storage.VersionStorage
	versionFile string
}

func NewSemverBumper(vs storage.VersionStorage, versionFile string) *SemverBumper {
	return &SemverBumper{vs, versionFile}
}

func (sb SemverBumper) BumpMajorVersion(preRelease string, metadata string) (*semver.Version, error) {
	v, err := sb.updateVersionFile(func(version *semver.Version) {
		version.BumpMajor()
	}, preRelease, metadata)

	if err != nil {
		return nil, errors.Mask(err)
	}

	return v, nil
}

func (sb SemverBumper) BumpMinorVersion(preRelease string, metadata string) (*semver.Version, error) {
	v, err := sb.updateVersionFile(func(version *semver.Version) {
		version.BumpMinor()
	}, preRelease, metadata)

	if err != nil {
		return nil, errors.Mask(err)
	}

	return v, nil
}

func (sb SemverBumper) BumpPatchVersion(preRelease string, metadata string) (*semver.Version, error) {
	v, err := sb.updateVersionFile(func(version *semver.Version) {
		version.BumpPatch()
	}, preRelease, metadata)

	if err != nil {
		return nil, errors.Mask(err)
	}

	return v, nil
}

func (sb SemverBumper) GetCurrentVersion() (*semver.Version, error) {
	currentVersion, err := sb.storage.ReadVersionFile(sb.versionFile)

	if err != nil {
		return nil, errors.Mask(err)
	}

	return currentVersion, nil
}

func (sb SemverBumper) InitVersion(initialVersion semver.Version) error {
	if sb.storage.VersionFileExists(sb.versionFile) {
		return errors.Newf("Version file exists. Looks like this project is already initialized.")
	}

	err := sb.storage.WriteVersionFile(sb.versionFile, initialVersion)

	if err != nil {
		errors.Mask(err)
	}

	return nil
}

func (sb SemverBumper) updateVersionFile(bumpCallback versionBumpCallback, preRelease string, metadata string) (*semver.Version, error) {
	currentVersion, err := sb.GetCurrentVersion()

	if err != nil {
		return nil, errors.Mask(err)
	}

	bumpedVersion := *currentVersion

	bumpCallback(&bumpedVersion)

	bumpedVersion.PreRelease = semver.PreRelease(preRelease)
	bumpedVersion.Metadata = metadata

	err = sb.storage.WriteVersionFile(sb.versionFile, bumpedVersion)

	if err != nil {
		return nil, errors.Mask(err)
	}

	return &bumpedVersion, nil
}
