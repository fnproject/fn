package server

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func init() {
	// gin is not nice by default, this can get set in logging initialization
	gin.SetMode(gin.ReleaseMode)

	// set machine id in init() before any packages are initialized that may use it
	// (you may change this to seed the id another way but be wary of package initialization)
	setMachineID()
}

func setMachineID() {
	port := uint16(getEnvInt(EnvPort, DefaultPort))
	addr := whoAmI().To4()
	if addr == nil {
		addr = net.ParseIP("127.0.0.1").To4()
		logrus.Warn("could not find non-local ipv4 address to use, using '127.0.0.1' for ids, if this is a cluster beware of duplicate ids!")
	}
	id.SetMachineIdHost(addr, port)
}

// whoAmI searches for a non-local address on any network interface, returning
// the first one it finds. it could be expanded to search eth0 or en0 only but
// to date this has been unnecessary.
func whoAmI() net.IP {
	ints, _ := net.Interfaces()
	for _, i := range ints {
		if i.Name == "docker0" || i.Name == "lo" {
			// not perfect
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			ip, _, err := net.ParseCIDR(a.String())
			if a.Network() == "ip+net" && err == nil && ip.To4() != nil {
				if !bytes.Equal(ip, net.ParseIP("127.0.0.1")) {
					return ip
				}
			}
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	} else if value, ok := os.LookupEnv(key + "_FILE"); ok {
		dat, err := ioutil.ReadFile(filepath.Clean(value))
		if err == nil {
			return string(dat)
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		// linter liked this better than if/else
		var err error
		var i int
		if i, err = strconv.Atoi(value); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"string": value, "environment_key": key}).Fatal("Failed to convert string to int")
		}
		return i
	} else if value, ok := os.LookupEnv(key + "_FILE"); ok {
		dat, err := ioutil.ReadFile(filepath.Clean(value))
		if err == nil {
			var err error
			var i int
			if i, err = strconv.Atoi(strings.TrimSpace(string(dat))); err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{"string": dat, "environment_key": key}).Fatal("Failed to convert string to int")
			}
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	var err error
	res := fallback
	if tmp := os.Getenv(key); tmp != "" {
		// if the returned value is not null it needs to be either an integral value in seconds or a parsable duration-format string
		res, err = time.ParseDuration(tmp)
		if err != nil {
			// try to parse an int
			s, perr := strconv.Atoi(tmp)
			if perr != nil {
				logrus.WithError(err).WithFields(logrus.Fields{"duration_string": tmp, "environment_key": key}).Fatal("Failed to parse duration from env")
			} else {
				res = time.Duration(s) * time.Second
			}
		}
	}
	return res
}

func contextWithSignal(ctx context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	newCTX, halt := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	go func() {
		for {
			select {
			case <-c:
				common.Logger(ctx).Info("Halting...")
				halt()
				return
			case <-ctx.Done():
				common.Logger(ctx).Info("Halting... Original server context canceled.")
				halt()
				return
			}
		}
	}()
	return newCTX, halt
}

// Installs a child process reaper if init process
func installChildReaper() {
	// assume responsibilities of init process if running as init process for Linux
	if runtime.GOOS != "linux" || os.Getpid() != 1 {
		return
	}

	var sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGCHLD)

	// we run this forever and leak a go routine. As init, we must
	// reap our children until the very end, so this is OK.
	go func() {
		for {
			<-sigs
			for {
				var status syscall.WaitStatus
				var rusage syscall.Rusage

				pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &rusage)
				// no children
				if pid <= 0 {
					break
				}

				logrus.Infof("Child terminated pid=%d err=%v status=%v usage=%v", pid, err, status, rusage)
			}
		}
	}()
}
