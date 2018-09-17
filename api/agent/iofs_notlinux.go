// +build !linux

package agent

import (
	"errors"
)

type tmpfsIOFS struct {
	directoryIOFS
}

func (t *tmpfsIOFS) Close() error {
	return t.directoryIOFS.Close()
}

func newTmpfsIOFS(cfg *Config) (*tmpfsIOFS, error) {
	return nil, errors.New("tmpfs IOFS not supported on macOS")
}

var _ iofs = &tmpfsIOFS{}
