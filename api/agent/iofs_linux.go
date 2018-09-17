package agent

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type tmpfsIOFS struct {
	directoryIOFS
}

func (t *tmpfsIOFS) Close() error {
	if err := unix.Unmount(t.AgentPath(), 0); err != nil {
		// At this point we don't have a lot of choice but to leak the directory and mount
		return err
	}
	return t.directoryIOFS.Close()
}

func newTmpfsIOFS(cfg *Config) (*tmpfsIOFS, error) {
	dirIOFS, err := newDirectoryIOFS(cfg)
	if err != nil {
		return nil, err
	}
	if err = unix.Mount("tmpfs", dirIOFS.AgentPath(), "tmpfs", uintptr(unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV), cfg.IOFSOpts); err != nil {
		// Best effort to clean up after failure. If the dirIOFS.Close() fails we're not going to see the error though...
		dirIOFS.Close()
		return nil, fmt.Errorf("cannot mount/create tmpfs at %s", dirIOFS.AgentPath())
	}
	return &tmpfsIOFS{*dirIOFS}, nil
}

var _ iofs = &tmpfsIOFS{}
