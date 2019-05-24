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

	handleErr := func(dir string) {
		if dir != "" {
			if err := os.RemoveAll(dir); err != nil {
				common.Logger(ctx).WithError(err).Error("failed to clean up iofs dir")
			}
		}
	}

	// create a tmpdir
	iofsAgentDir, err := ioutil.TempDir(cfg.IOFSAgentPath, "iofs")
	if err != nil {
		handleErr(iofsAgentDir)
		return nil, fmt.Errorf("cannot create tmpdir for iofs: %v", err)
	}

	if !cfg.DisableUnprivilegedContainers && !cfg.IOFSEnableTmpfs {
		err := os.Chmod(iofsAgentDir, 0777) // #nosec G302
		if err != nil {
			handleErr(iofsAgentDir)
			return nil, fmt.Errorf("cannot change iofs mod: %v", err)
		}
	}

	ret := &directoryIOFS{iofsAgentDir, iofsAgentDir}

	if cfg.IOFSMountRoot != "" {
		iofsRelPath, err := filepath.Rel(cfg.IOFSAgentPath, iofsAgentDir)
		if err != nil {
			handleErr(iofsAgentDir)
			return nil, fmt.Errorf("cannot relativise iofs path: %v", err)
		}
		ret.dockerPath = filepath.Join(cfg.IOFSMountRoot, iofsRelPath)
	}

	return ret, nil
}

var _ iofs = &directoryIOFS{}
