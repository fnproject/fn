package agent

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func platformSupportsTmpfs() bool {
	return true
}

type tmpfsIOFS struct {
	directoryIOFS
}

func (t *tmpfsIOFS) Close() error {
	if err := unix.Unmount(t.AgentPath(), 0); err != nil {
		return err
	}
	return t.directoryIOFS.Close()
}

func newTmpfsIOFS(cfg *Config) (*tmpfsIOFS, error) {
	dirIOFS, err := newDirectoryIOFS(cfg)
	if err != nil {
		return nil, err
	}
	err = unix.Mount("tmpfs", dirIOFS.AgentPath(), "tmpfs", uintptr(unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV), cfg.IOFSOpts)
	if err != nil {
		return nil, fmt.Errorf("cannot mount/create tmpfs at %s", dirIOFS.AgentPath())
	}
	return &tmpfsIOFS{*dirIOFS}, nil
}

var _ iofs = &tmpfsIOFS{}
