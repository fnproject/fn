package agent

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type iofs interface {
	io.Closer
	AgentPath() string
	DockerPath() string
}

type noopIOFS struct {
}

func (n *noopIOFS) AgentPath() string {
	return ""
}

func (n *noopIOFS) DockerPath() string {
	return ""
}

func (n *noopIOFS) Close() error {
	return nil
}

type directoryIOFS struct {
	agentPath  string
	dockerPath string
}

func (d *directoryIOFS) AgentPath() string {
	return d.agentPath
}

func (d *directoryIOFS) DockerPath() string {
	return d.dockerPath
}

func (d *directoryIOFS) Close() error {
	err := os.RemoveAll(d.agentPath)
	if err != nil {
		return err
	}
	return nil
}

func newDirectoryIOFS(cfg *Config) (*directoryIOFS, error) {
	// XXX(reed): need to ensure these are cleaned up if any of these ops in here fail...

	dir := cfg.IOFSAgentPath
	if dir == "" {
		// /tmp should be a memory backed filesystem, where we can get user perms
		// on the socket file (fdks must give write permissions to users on sock).
		// /var/run is root only, hence this...
		dir = "/tmp"
	}

	// create a tmpdir
	iofsAgentDir, err := ioutil.TempDir(dir, "iofs")
	if err != nil {
		return nil, fmt.Errorf("cannot create tmpdir for iofs: %v", err)
	}

	if cfg.IOFSMountRoot != "" {
		iofsRelPath, err := filepath.Rel(dir, iofsAgentDir)
		if err != nil {
			return nil, fmt.Errorf("cannot relativise iofs path: %v", err)
		}
		iofsDockerDir := filepath.Join(cfg.IOFSMountRoot, iofsRelPath)
		return &directoryIOFS{iofsAgentDir, iofsDockerDir}, nil
	}
	return &directoryIOFS{iofsAgentDir, iofsAgentDir}, nil
}

var _ iofs = &directoryIOFS{}
