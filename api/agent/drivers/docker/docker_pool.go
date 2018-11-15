package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
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

type PoolTaskStateType int

const (
	PoolTaskStateInit  PoolTaskStateType = iota // initializing
	PoolTaskStateReady                          // ready to be run
)

const (
	LimitPerSec = 10
	LimitBurst  = 20
)

type poolTask struct {
	id      string
	image   string
	cmd     string
	netMode string
	state   PoolTaskStateType
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
func (c *poolTask) TmpFsSize() uint64                                { return 0 }
func (c *poolTask) Extensions() map[string]string                    { return nil }
func (c *poolTask) LoggerConfig() drivers.LoggerConfig               { return drivers.LoggerConfig{} }
func (c *poolTask) WriteStat(ctx context.Context, stat drivers.Stat) {}
func (c *poolTask) UDSAgentPath() string                             { return "" }
func (c *poolTask) UDSDockerPath() string                            { return "" }
func (c *poolTask) UDSDockerDest() string                            { return "" }

type dockerPoolItem struct {
	id     string
	cancel func()
}

type dockerPool struct {
	lock      sync.Mutex
	inuse     map[string]dockerPoolItem
	free      []dockerPoolItem
	limiter   *rate.Limiter
	cancel    func()
	wg        sync.WaitGroup
	isRecycle bool
}

type DockerPoolStats struct {
	inuse int
	free  int
}

type DockerPool interface {
	// fetch a pre-allocated free id from the pool
	// may return too busy error.
	AllocPoolId() (string, error)

	// Release the id back to the pool
	FreePoolId(id string)

	// stop and terminate the pool
	Close() error

	// returns inuse versus free
	Usage() DockerPoolStats
}

func NewDockerPool(conf drivers.Config, driver *DockerDriver) DockerPool {

	// Docker pool is an optimization & feature only for Linux
	if runtime.GOOS != "linux" || conf.PreForkPoolSize == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	log := common.Logger(ctx)
	log.Error("WARNING: Experimental Prefork Docker Pool Enabled")

	pool := &dockerPool{
		inuse:   make(map[string]dockerPoolItem, conf.PreForkPoolSize),
		free:    make([]dockerPoolItem, 0, conf.PreForkPoolSize),
		limiter: rate.NewLimiter(LimitPerSec, LimitBurst),
		cancel:  cancel,
	}

	if conf.PreForkUseOnce != 0 {
		pool.isRecycle = true
	}

	networks := strings.Fields(conf.PreForkNetworks)
	if len(networks) == 0 {
		networks = append(networks, "")
	}

	pool.wg.Add(1 + int(conf.PreForkPoolSize))
	pullGate := make(chan struct{}, 1)

	for i := 0; i < int(conf.PreForkPoolSize); i++ {

		task := &poolTask{
			id:      fmt.Sprintf("%d_prefork_%s", i, id.New().String()),
			image:   conf.PreForkImage,
			cmd:     conf.PreForkCmd,
			netMode: networks[i%len(networks)],
		}

		go pool.nannyContainer(ctx, driver, task, pullGate)
	}

	go pool.prepareImage(ctx, driver, conf.PreForkImage, pullGate)
	return pool
}

func (pool *dockerPool) Close() error {
	pool.cancel()
	pool.wg.Wait()
	return nil
}

func (pool *dockerPool) performInitState(ctx context.Context, driver *DockerDriver, task *poolTask) {

	log := common.Logger(ctx).WithFields(logrus.Fields{"id": task.Id(), "net": task.netMode})

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
			NetworkMode: task.netMode,
		},
		Context: ctx,
	}

	removeOpts := docker.RemoveContainerOptions{
		ID:            task.Id(),
		Force:         true,
		RemoveVolumes: true,
		Context:       ctx,
	}

	// ignore failure here
	driver.docker.RemoveContainer(removeOpts)

	_, err := driver.docker.CreateContainer(containerOpts)
	if err != nil {
		log.WithError(err).Info("prefork pool container create failed")
		return
	}

	task.state = PoolTaskStateReady
}

func (pool *dockerPool) performReadyState(ctx context.Context, driver *DockerDriver, task *poolTask) {

	log := common.Logger(ctx).WithFields(logrus.Fields{"id": task.Id(), "net": task.netMode})

	killOpts := docker.KillContainerOptions{
		ID:      task.Id(),
		Context: ctx,
	}

	defer func() {
		err := driver.docker.KillContainer(killOpts)
		if err != nil {
			log.WithError(err).Info("prefork pool container kill failed")
			task.state = PoolTaskStateInit
		}
	}()

	err := driver.docker.StartContainerWithContext(task.Id(), nil, ctx)
	if err != nil {
		log.WithError(err).Info("prefork pool container start failed")
		task.state = PoolTaskStateInit
		return
	}

	log.Debug("prefork pool container ready")

	// IMPORTANT: container is now up and running. Register it to make it
	// available for function containers.
	ctx, cancel := context.WithCancel(ctx)

	pool.register(task.Id(), cancel)
	exitCode, err := driver.docker.WaitContainerWithContext(task.Id(), ctx)
	pool.unregister(task.Id())

	if ctx.Err() == nil {
		log.WithError(err).Infof("prefork pool container exited exit_code=%d", exitCode)
		task.state = PoolTaskStateInit
	}
}

func (pool *dockerPool) performTeardown(ctx context.Context, driver *DockerDriver, task *poolTask) {
	removeOpts := docker.RemoveContainerOptions{
		ID:            task.Id(),
		Force:         true,
		RemoveVolumes: true,
		Context:       context.Background(),
	}

	driver.docker.RemoveContainer(removeOpts)
}

func (pool *dockerPool) prepareImage(ctx context.Context, driver *DockerDriver, img string, pullGate chan struct{}) {
	defer pool.wg.Done()
	defer close(pullGate)

	log := common.Logger(ctx)

	imgReg, imgRepo, imgTag := drivers.ParseImage(img)
	opts := docker.PullImageOptions{Repository: path.Join(imgReg, imgRepo), Tag: imgTag, Context: ctx}
	config := findRegistryConfig(imgReg, driver.auths)

	for ctx.Err() != nil {
		err := pool.limiter.Wait(ctx)
		if err != nil {
			// should not really happen unless ctx has a deadline or burst is 0.
			log.WithError(err).Fatal("prefork pool rate limiter failed")
		}

		_, err = driver.docker.InspectImage(ctx, img)
		if err == nil {
			return
		}
		if err != docker.ErrNoSuchImage {
			log.WithError(err).Fatal("prefork pool image inspect failed")
		}

		err = driver.docker.PullImage(opts, *config)
		if err == nil {
			return
		}

		log.WithError(err).Error("Failed to pull image")
	}
}

func (pool *dockerPool) nannyContainer(ctx context.Context, driver *DockerDriver, task *poolTask, pullGate chan struct{}) {
	defer pool.performTeardown(ctx, driver, task)
	defer pool.wg.Done()

	// wait for image pull
	select {
	case <-ctx.Done():
	case <-pullGate:
	}

	log := common.Logger(ctx).WithFields(logrus.Fields{"id": task.Id(), "net": task.netMode})

	// We spin forever, keeping the pool resident and running at all times.
	for ctx.Err() == nil {

		if task.state != PoolTaskStateReady {
			err := pool.limiter.Wait(ctx)
			if err != nil {
				// should not really happen unless ctx has a deadline or burst is 0.
				log.WithError(err).Fatal("prefork pool rate limiter failed")
				break
			}
		}

		if task.state != PoolTaskStateReady {
			pool.performInitState(ctx, driver, task)
		}

		if task.state == PoolTaskStateReady {
			pool.performReadyState(ctx, driver, task)
		}
	}
}

func (pool *dockerPool) register(id string, cancel func()) {
	item := dockerPoolItem{
		id:     id,
		cancel: cancel,
	}

	pool.lock.Lock()
	pool.free = append(pool.free, item)
	pool.lock.Unlock()
}

func (pool *dockerPool) unregister(id string) {
	pool.lock.Lock()

	_, ok := pool.inuse[id]
	if ok {
		delete(pool.inuse, id)
	} else {
		for i := 0; i < len(pool.free); i += 1 {
			if pool.free[i].id == id {
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
	pool.inuse[id.id] = id

	return id.id, nil
}

func (pool *dockerPool) FreePoolId(id string) {

	isRecycle := pool.isRecycle

	pool.lock.Lock()

	item, ok := pool.inuse[id]
	if ok {
		if item.cancel != nil && isRecycle {
			item.cancel()
		}
		delete(pool.inuse, id)
		if !isRecycle {
			pool.free = append(pool.free, item)
		}
	}

	pool.lock.Unlock()
}

func (pool *dockerPool) Usage() DockerPoolStats {
	var stats DockerPoolStats
	pool.lock.Lock()

	stats.inuse = len(pool.inuse)
	stats.free = len(pool.free)

	pool.lock.Unlock()
	return stats
}
