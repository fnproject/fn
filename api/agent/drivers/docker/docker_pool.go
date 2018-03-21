package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"

	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Prefork Pool is used in namespace optimizations to avoid creating and
// tearing down namespaces with every function container run. Instead, we reuse
// already available namespaces from these running container instances. These
// containers are not designed to run anything but placeholders for namespaces
// such as a minimal busybox container with 'tail -f /dev/null' which blocks
// forever. In other words, every function container is paired with a pool buddy
// where pool buddy provides already creates namespaces. These are currently
// network and user namespaces, but perhaps can be extended to also use pid and ipc.
// (see docker.go Prepare() on how this is currently being used.)
// Currently the pool is a set size and it does not grow on demand.

var (
	ErrorPoolEmpty = errors.New("docker pre fork pool empty")
)

const (
	LimitPerSec = 10
	LimitBurst  = 20

	DefaultImage = "busybox"
	DefaultCmd   = "tail -f /dev/null"
)

type poolTask struct {
	id    string
	image string
	cmd   string
}

func (c *poolTask) Id() string                                       { return c.id }
func (c *poolTask) Command() string                                  { return c.cmd }
func (c *poolTask) Input() io.Reader                                 { return nil }
func (c *poolTask) Logger() (io.Writer, io.Writer)                   { return nil, nil }
func (c *poolTask) Volumes() [][2]string                             { return nil }
func (c *poolTask) WorkDir() string                                  { return "" }
func (c *poolTask) Close()                                           {}
func (c *poolTask) Image() string                                    { return c.image }
func (c *poolTask) Timeout() time.Duration                           { return 0 }
func (c *poolTask) EnvVars() map[string]string                       { return nil }
func (c *poolTask) Memory() uint64                                   { return 0 }
func (c *poolTask) CPUs() uint64                                     { return 0 }
func (c *poolTask) FsSize() uint64                                   { return 0 }
func (c *poolTask) WriteStat(ctx context.Context, stat drivers.Stat) {}

type dockerPool struct {
	lock    sync.Mutex
	inuse   map[string]struct{}
	free    []string
	limiter *rate.Limiter
}

type DockerPool interface {
	// fetch a pre-allocated free id from the pool
	// may return too busy error
	AllocPoolId() (string, error)

	// Release the id back to the pool
	FreePoolId(id string)
}

func NewDockerPool(conf drivers.Config, driver *DockerDriver) DockerPool {

	// Docker pool is an optimization & feature only for Linux
	if runtime.GOOS != "linux" {
		return nil
	}

	pool := &dockerPool{
		inuse:   make(map[string]struct{}, conf.PreForkPoolSize),
		free:    make([]string, 0, conf.PreForkPoolSize),
		limiter: rate.NewLimiter(LimitPerSec, LimitBurst),
	}

	ctx := context.Background()
	for i := 0; i < cap(pool.free); i++ {

		task := &poolTask{
			id:    fmt.Sprintf("%d_prefork_%s", i, id.New().String()),
			image: DefaultImage,
			cmd:   DefaultCmd,
		}

		if conf.PreForkImage != "" {
			task.image = conf.PreForkImage
		}
		if conf.PreForkCmd != "" {
			task.cmd = conf.PreForkCmd
		}

		go pool.nannyContainer(ctx, driver, task)
	}

	return pool
}

func (pool *dockerPool) nannyContainer(ctx context.Context, driver *DockerDriver, task *poolTask) {

	log := common.Logger(ctx).WithFields(logrus.Fields{"name": task.Id()})

	containerOpts := docker.CreateContainerOptions{
		Name: task.Id(),
		Config: &docker.Config{
			Cmd:          strings.Fields(task.Command()),
			Hostname:     task.Id(),
			Image:        task.Image(),
			Volumes:      map[string]struct{}{},
			OpenStdin:    false,
			AttachStdout: false,
			AttachStderr: false,
			AttachStdin:  false,
			StdinOnce:    false,
		},
		HostConfig: &docker.HostConfig{
			LogConfig: docker.LogConfig{
				Type: "none",
			},
		},
		Context: ctx,
	}

	removeOpts := docker.RemoveContainerOptions{
		ID:            task.Id(),
		Force:         true,
		RemoveVolumes: true,
		Context:       ctx,
	}

	// We spin forever, keeping the pool resident and running at all times.
	for {
		err := pool.limiter.Wait(ctx)
		if err != nil {
			// should not really happen unless ctx has a deadline or burst is 0.
			log.WithError(err).Info("prefork pool rate limiter failed")
			break
		}

		// Let's try to clean up any left overs
		err = driver.docker.RemoveContainer(removeOpts)
		if err != nil {
			log.WithError(err).Info("prefork pool container remove failed (this is probably OK)")
		}

		err = driver.ensureImage(ctx, task)
		if err != nil {
			log.WithError(err).Info("prefork pool image pull failed")
			continue
		}

		_, err = driver.docker.CreateContainer(containerOpts)
		if err != nil {
			log.WithError(err).Info("prefork pool container create failed")
			continue
		}

		err = driver.docker.StartContainerWithContext(task.Id(), nil, ctx)
		if err != nil {
			log.WithError(err).Info("prefork pool container start failed")
			continue
		}

		log.Debug("prefork pool container ready")

		// IMPORTANT: container is now up and running. Register it to make it
		// available for function containers.
		pool.register(task.Id())

		// We are optimistic here where provided image and command really blocks
		// and runs forever.
		exitCode, err := driver.docker.WaitContainerWithContext(task.Id(), ctx)

		// IMPORTANT: We have exited. This window is potentially very destructive, as any new
		// function containers created during this window will fail. We must immediately
		// proceed to unregister ourself to avoid further issues.
		pool.unregister(task.Id())

		log.WithError(err).Infof("prefork pool container exited exit_code=%d", exitCode)
	}
}

func (pool *dockerPool) register(id string) {
	pool.lock.Lock()
	pool.free = append(pool.free, id)
	pool.lock.Unlock()
}

func (pool *dockerPool) unregister(id string) {
	pool.lock.Lock()

	_, ok := pool.inuse[id]
	if ok {
		delete(pool.inuse, id)
	} else {
		for i := 0; i < len(pool.free); i += 1 {
			if pool.free[i] == id {
				pool.free = append(pool.free[:i], pool.free[i+1:]...)
				break
			}
		}
	}

	pool.lock.Unlock()
}

func (pool *dockerPool) AllocPoolId() (string, error) {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	// We currently do not grow the pool if we run out of pre-forked containers
	if len(pool.free) == 0 {
		return "", ErrorPoolEmpty
	}

	id := pool.free[len(pool.free)-1]
	pool.free = pool.free[:len(pool.free)-1]
	pool.inuse[id] = struct{}{}

	return id, nil
}

func (pool *dockerPool) FreePoolId(id string) {
	pool.lock.Lock()

	_, ok := pool.inuse[id]
	if ok {
		delete(pool.inuse, id)
		pool.free = append(pool.free, id)
	}

	pool.lock.Unlock()
}
