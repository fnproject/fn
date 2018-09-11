// +build linux

package agent

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

	opts := "size=1k,nr_inodes=8,mode=0777"

	// under tmpdir, create tmpfs
	err = syscall.Mount("tmpfs", iofsDir, "tmpfs", uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV), opts)
	if err != nil {
		return "", fmt.Errorf("cannot mount/create tmpfs=%s", iofsDir)
	}

	return iofsDir, nil
}
