// +build !linux

package agent

import (
	"context"
	"errors"
)

type tmpfsIOFS struct {
	directoryIOFS
}

func (t *tmpfsIOFS) Close() error {
	return t.directoryIOFS.Close()
}

func newTmpfsIOFS(ctx context.Context, cfg *Config) (*tmpfsIOFS, error) {
	return nil, errors.New("tmpfs IOFS not supported on macOS")
}

var _ iofs = &tmpfsIOFS{}
