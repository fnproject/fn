package agent

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/fnproject/fn/api/common"
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

func newDirectoryIOFS(ctx context.Context, cfg *Config) (*directoryIOFS, error) {
	dir := cfg.IOFSAgentPath

	// create a tmpdir
	iofsAgentDir, err := ioutil.TempDir(dir, "iofs")
	if err != nil {
		if err := os.RemoveAll(iofsAgentDir); err != nil {
			common.Logger(ctx).WithError(err).Error("failed to clean up iofs dir")
		}
		return nil, fmt.Errorf("cannot create tmpdir for iofs: %v", err)
	}

	if cfg.IOFSMountRoot != "" {
		iofsRelPath, err := filepath.Rel(dir, iofsAgentDir)
		if err != nil {
			if err := os.RemoveAll(iofsAgentDir); err != nil {
				common.Logger(ctx).WithError(err).Error("failed to clean up iofs dir")
			}
			return nil, fmt.Errorf("cannot relativise iofs path: %v", err)
		}
		iofsDockerDir := filepath.Join(cfg.IOFSMountRoot, iofsRelPath)
		return &directoryIOFS{iofsAgentDir, iofsDockerDir}, nil
	}
	return &directoryIOFS{iofsAgentDir, iofsAgentDir}, nil
}

var _ iofs = &directoryIOFS{}
