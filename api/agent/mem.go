package agent

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func getAvailableMemory() uint64 {
	const tooBig = 322122547200 // #300GB or 0, biggest aws instance is 244GB

	var availableMemory uint64 = tooBig
	if runtime.GOOS == "linux" {
		var err error
		availableMemory, err = checkCgroup()
		if err != nil {
			logrus.WithError(err).Error("Error checking for cgroup memory limits, falling back to host memory available..")
		}
		if availableMemory >= tooBig || availableMemory <= 0 {
			// Then -m flag probably wasn't set, so use max available on system
			availableMemory, err = checkProc()
			if availableMemory >= tooBig || availableMemory <= 0 {
				logrus.WithError(err).Fatal("Cannot get the proper memory information to size server. You must specify the maximum available memory by passing the -m command with docker run when starting the server via docker, eg:  `docker run -m 2G ...`")
			}
		}
	} else {
		// This still lets 10-20 functions execute concurrently assuming a 2GB machine.
		availableMemory = 2 * 1024 * 1024 * 1024
	}

	logrus.WithFields(logrus.Fields{"ram": availableMemory}).Info("available memory")

	return availableMemory
}

func checkCgroup() (uint64, error) {
	f, err := os.Open("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	limBytes := string(b)
	limBytes = strings.TrimSpace(limBytes)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(limBytes, 10, 64)
}

var errCantReadMemInfo = errors.New("Didn't find MemAvailable in /proc/meminfo, kernel is probably < 3.14")

func checkProc() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		b := scanner.Text()
		if !strings.HasPrefix(b, "MemAvailable") {
			continue
		}

		// expect form:
		// MemAvailable: 1234567890 kB
		tri := strings.Fields(b)
		if len(tri) != 3 {
			return 0, fmt.Errorf("MemAvailable line has unexpected format: %v", b)
		}

		c, err := strconv.ParseUint(tri[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("Could not parse MemAvailable: %v", b)
		}
		switch tri[2] { // convert units to bytes
		case "kB":
			c *= 1024
		case "MB":
			c *= 1024 * 1024
		default:
			return 0, fmt.Errorf("Unexpected units for MemAvailable in /proc/meminfo, need kB or MB, got: %v", tri[2])
		}
		return c, nil
	}

	return 0, errCantReadMemInfo
}
