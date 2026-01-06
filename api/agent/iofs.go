package agent

import (
	"context"
	"fmt"
	"github.com/fnproject/fn/api/common"
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

func newDirectoryIOFS(ctx context.Context, cfg *Config) (*directoryIOFS, error) {

	handleErr := func(dir string) {
		if dir != "" {
			if err := os.RemoveAll(dir); err != nil {
				common.Logger(ctx).WithError(err).Error("failed to clean up iofs dir")
			}
		}
	}

	iofsAgentPath := ""
	if len(cfg.IOFSAgentPath) > 0 {
		iofsAgentPath = cfg.IOFSAgentPath
	} else {
		// iofsPath: /<host tmp dir>/iofs
		//   Our system integration test runs Fn Server as plain process (instead of as a docker container)
		//   and not specify cfg.IOFSAgentPath.
		//   When Fn Server spawns a Fn container, the iofsPath will be mapped to /tmp/iofs in Fn container
		//   for Fn container to access the socket
		iofsPath := filepath.Join(os.TempDir(), "iofs")

		err := os.Mkdir(iofsPath, 0755) // #nosec G301
		if err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("cannot create iofs directory under TempDir: %v", err)
		}
		iofsAgentPath = iofsPath
	}

	// create a tmpdir
	iofsAgentDir, err := ioutil.TempDir(iofsAgentPath, "iofs")
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

	ret := &directoryIOFS{iofsAgentDir, iofsAgentPath}

	if cfg.IOFSMountRoot != "" {
		// cfg.IOFSMountRoot is the source path on host vm hosting the unix socket file required for FDK/Fn Server
		// if the value is specified, it will map to /tmp/iofs in the Fn container. It allows user to override with
		// podman/rancher volume instead of using their $HOME directory
		ret.dockerPath = cfg.IOFSMountRoot
	}

	return ret, nil
}

var _ iofs = &directoryIOFS{}
