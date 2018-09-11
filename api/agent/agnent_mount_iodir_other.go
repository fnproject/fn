// +build !linux

package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

func createIOFS(cfg *Config) (string, error) {
	// XXX(reed): need to ensure these are cleaned up if any of these ops in here fail...

	dir := cfg.IOFSPath
	if dir == "" {
		// XXX(reed): figure out a sane default here...
		pwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get pwd to create iofs: %v", err)
		}
		dir = path.Join(pwd, "tmp")

		err = os.MkdirAll(dir, 0777)
		if err != nil {
			return "", fmt.Errorf("cannot create directory for iofs: %v", err)
		}
	}

	// create a tmpdir
	iofsDir, err := ioutil.TempDir(dir, "iofs")
	if err != nil {
		return "", fmt.Errorf("cannot create tmpdir for iofs: %v", err)
	}

	err = os.Mkdir(iofsDir, 0777)
	if err != nil {
		return "", err
	}

	return iofsDir, nil
}
