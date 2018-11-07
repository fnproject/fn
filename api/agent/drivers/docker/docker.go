package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/trace"

	"github.com/coreos/go-semver/semver"
	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
)

// A drivers.ContainerTask should implement the Auther interface if it would
// like to use not-necessarily-public docker images for any or all task
// invocations.
type Auther interface {
	// DockerAuth should return docker auth credentials that will authenticate
	// against a docker registry for a given drivers.ContainerTask.Image(). An
	// error may be returned which will cause the task not to be run, this can be
	// useful for an implementer to do things like testing auth configurations
	// before returning them; e.g. if the implementer would like to impose
	// certain restrictions on images or if credentials must be acquired right
	// before runtime and there's an error doing so. If these credentials don't
	// work, the docker pull will fail and the task will be set to error status.
	DockerAuth() (*docker.AuthConfiguration, error)
}

type runResult struct {
	err    error
	status string
}

type driverAuthConfig struct {
	auth       docker.AuthConfiguration
	subdomains map[string]bool
}

func (r *runResult) Error() error   { return r.err }
func (r *runResult) Status() string { return r.status }

type DockerDriver struct {
	cancel   func()
	conf     drivers.Config
	docker   dockerClient // retries on *docker.Client, restricts ad hoc *docker.Client usage / retries
	hostname string
	auths    map[string]driverAuthConfig
	pool     DockerPool
	// protects networks map
	networksLock sync.Mutex
	networks     map[string]uint64
}

// implements drivers.Driver
func NewDocker(conf drivers.Config) *DockerDriver {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't resolve hostname")
	}

	auths, err := registryFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't initialize registry")
	}

	ctx, cancel := context.WithCancel(context.Background())
	driver := &DockerDriver{
		cancel:   cancel,
		conf:     conf,
		docker:   newClient(ctx, conf.MaxRetries),
		hostname: hostname,
		auths:    auths,
	}

	if conf.ServerVersion != "" {
		err = checkDockerVersion(driver, conf.ServerVersion)
		if err != nil {
			logrus.WithError(err).Fatal("docker version error")
		}
	}

	if conf.PreForkPoolSize != 0 {
		driver.pool = NewDockerPool(conf, driver)
	}

	nets := strings.Fields(conf.DockerNetworks)
	if len(nets) > 0 {
		driver.networks = make(map[string]uint64, len(nets))
		for _, net := range nets {
			driver.networks[net] = 0
		}
	}

	if conf.DockerLoadFile != "" {
		err = loadDockerImages(driver, conf.DockerLoadFile)
		if err != nil {
			logrus.WithError(err).Fatalf("cannot load docker images in %s", conf.DockerLoadFile)
		}
	}

	return driver
}

func checkDockerVersion(driver *DockerDriver, expected string) error {
	info, err := driver.docker.Info(context.Background())
	if err != nil {
		return err
	}

	actual, err := semver.NewVersion(info.ServerVersion)
	if err != nil {
		return err
	}

	wanted, err := semver.NewVersion(expected)
	if err != nil {
		return err
	}

	if actual.Compare(*wanted) < 0 {
		return fmt.Errorf("docker version is too old. Required: %s Found: %s", expected, info.ServerVersion)
	}

	return nil
}

func loadDockerImages(driver *DockerDriver, filePath string) error {
	ctx, log := common.LoggerWithFields(context.Background(), logrus.Fields{"stack": "loadDockerImages"})
	log.Infof("Loading docker images from %v", filePath)
	return driver.docker.LoadImages(ctx, filePath)
}

func (drv *DockerDriver) Close() error {
	var err error
	if drv.pool != nil {
		err = drv.pool.Close()
	}
	if drv.cancel != nil {
		drv.cancel()
	}
	return err
}

func (drv *DockerDriver) pickPool(ctx context.Context, c *cookie) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "tryUsePool"})

	if drv.pool == nil || c.opts.HostConfig.NetworkMode != "" {
		return
	}

	id, err := drv.pool.AllocPoolId()
	if err != nil {
		log.WithError(err).Error("Could not fetch pre fork pool container")
		return
	}

	// We are able to fetch a container from pool. Now, use its
	// network, ipc and pid namespaces.
	c.opts.HostConfig.NetworkMode = fmt.Sprintf("container:%s", id)
	//c.opts.HostConfig.IpcMode = linker
	//c.opts.HostConfig.PidMode = linker
	c.poolId = id
}

func (drv *DockerDriver) unpickPool(c *cookie) {
	if c.poolId != "" && drv.pool != nil {
		drv.pool.FreePoolId(c.poolId)
	}
}

func (drv *DockerDriver) pickNetwork(c *cookie) {

	if len(drv.networks) == 0 || c.opts.HostConfig.NetworkMode != "" {
		return
	}

	var id string
	min := uint64(math.MaxUint64)

	drv.networksLock.Lock()
	for key, val := range drv.networks {
		if val < min {
			id = key
			min = val
		}
	}
	drv.networks[id]++
	drv.networksLock.Unlock()

	c.opts.HostConfig.NetworkMode = id
	c.netId = id
}

func (drv *DockerDriver) unpickNetwork(c *cookie) {
	if c.netId != "" {
		c.drv.networksLock.Lock()
		c.drv.networks[c.netId]--
		c.drv.networksLock.Unlock()
	}
}

func (drv *DockerDriver) CreateCookie(ctx context.Context, task drivers.ContainerTask) (drivers.Cookie, error) {

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "CreateCookie"})

	stdinOn := task.Input() != nil
	// XXX(reed): we can do same with stderr/stdout

	opts := docker.CreateContainerOptions{
		Name: task.Id(),
		Config: &docker.Config{
			Image:        task.Image(),
			OpenStdin:    stdinOn,
			StdinOnce:    stdinOn,
			AttachStdin:  stdinOn,
			AttachStdout: true,
			AttachStderr: true,
		},
		HostConfig: &docker.HostConfig{
			ReadonlyRootfs: drv.conf.EnableReadOnlyRootFs,
			Init:           true,
		},
	}

	cookie := &cookie{
		opts: opts,
		task: task,
		drv:  drv,
	}

	cookie.configureLogger(log)
	cookie.configureMem(log)
	cookie.configureCmd(log)
	cookie.configureEnv(log)
	cookie.configureCPU(log)
	cookie.configureFsSize(log)
	cookie.configureTmpFs(log)
	cookie.configureVolumes(log)
	cookie.configureWorkDir(log)
	cookie.configureIOFS(log)

	// Order is important, if pool is enabled, it overrides pick network
	drv.pickPool(ctx, cookie)
	drv.pickNetwork(cookie)

	// Order is important, Hostname doesn't play well with Network config
	cookie.configureHostname(log)

	return cookie, nil
}

func (drv *DockerDriver) PrepareCookie(ctx context.Context, c drivers.Cookie) error {

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "PrepareCookie"})
	cookie, ok := c.(*cookie)
	if !ok {
		return errors.New("unknown cookie implementation")
	}

	err := drv.ensureImage(ctx, cookie.task)
	if err != nil {
		return err
	}

	// here let's assume we have created container, logically this should be after 'CreateContainer', but we
	// are not 100% sure that *any* failure to CreateContainer does not ever leave a container around especially
	// going through fsouza+docker-api.
	cookie.isCreated = true

	cookie.opts.Context = ctx
	_, err = drv.docker.CreateContainer(cookie.opts)
	if err != nil {
		// since we retry under the hood, if the container gets created and retry fails, we can just ignore error
		if err != docker.ErrContainerAlreadyExists {
			log.WithFields(logrus.Fields{"call_id": cookie.task.Id(), "command": cookie.opts.Config.Cmd, "memory": cookie.opts.Config.Memory,
				"cpu_quota": cookie.task.CPUs(), "hostname": cookie.opts.Config.Hostname, "name": cookie.opts.Name,
				"image": cookie.opts.Config.Image, "volumes": cookie.opts.Config.Volumes, "binds": cookie.opts.HostConfig.Binds, "container": cookie.opts.Name,
			}).WithError(err).Error("Could not create container")
			// NOTE: if the container fails to create we don't really want to show to user since they aren't directly configuring the container
			return err
		}
	}

	// discard removal error
	return nil
}

func (drv *DockerDriver) removeContainer(ctx context.Context, container string) error {
	err := drv.docker.RemoveContainer(docker.RemoveContainerOptions{
		ID: container, Force: true, RemoveVolumes: true, Context: ctx})

	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"container": container}).Error("error removing container")
	}
	return nil
}

func (drv *DockerDriver) ensureImage(ctx context.Context, task drivers.ContainerTask) error {
	reg, repo, tag := drivers.ParseImage(task.Image())

	// ask for docker creds before looking for image, as the tasker may need to
	// validate creds even if the image is downloaded.

	config := findRegistryConfig(reg, drv.auths)

	if task, ok := task.(Auther); ok {
		var err error
		_, span := trace.StartSpan(ctx, "docker_auth")
		authConfig, err := task.DockerAuth()
		span.End()
		if err != nil {
			return err
		}
		if authConfig != nil {
			config = authConfig
		}
	}

	globalRepo := path.Join(reg, repo)

	// see if we already have it, if not, pull it
	_, err := drv.docker.InspectImage(ctx, task.Image())
	if err == docker.ErrNoSuchImage {
		err = drv.pullImage(ctx, task, *config, globalRepo, tag)
	}

	return err
}

func (drv *DockerDriver) pullImage(ctx context.Context, task drivers.ContainerTask, config docker.AuthConfiguration, globalRepo, tag string) error {
	log := common.Logger(ctx)

	log.WithFields(logrus.Fields{"registry": config.ServerAddress, "username": config.Username, "image": task.Image()}).Info("Pulling image")

	err := drv.docker.PullImage(docker.PullImageOptions{Repository: globalRepo, Tag: tag, Context: ctx}, config)
	if err != nil {
		log.WithFields(logrus.Fields{"registry": config.ServerAddress, "username": config.Username, "image": task.Image()}).WithError(err).Error("Failed to pull image")

		// TODO need to inspect for hub or network errors and pick; for now, assume
		// 500 if not a docker error
		msg := err.Error()
		code := http.StatusInternalServerError
		if dErr, ok := err.(*docker.Error); ok {
			msg = dockerMsg(dErr)
			code = dErr.Status // 401/404
		}

		return models.NewAPIError(code, fmt.Errorf("Failed to pull image '%s': %s", task.Image(), msg))
	}

	return nil
}

// removes docker err formatting: 'API Error (code) {"message":"..."}'
func dockerMsg(derr *docker.Error) string {
	// derr.Message is a JSON response from docker, which has a "message" field we want to extract if possible.
	// this is pretty lame, but it is what it is
	var v struct {
		Msg string `json:"message"`
	}

	err := json.Unmarshal([]byte(derr.Message), &v)
	if err != nil {
		// If message was not valid JSON, the raw body is still better than nothing.
		return derr.Message
	}
	return v.Msg
}

// Run executes the docker container. If task runs, drivers.RunResult will be returned. If something fails outside the task (ie: Docker), it will return error.
// The docker driver will attempt to cast the task to a Auther. If that succeeds, private image support is available. See the Auther interface for how to implement this.
func (drv *DockerDriver) run(ctx context.Context, container string, task drivers.ContainerTask) (drivers.WaitResult, error) {

	mwOut, mwErr := task.Logger()
	successChan := make(chan struct{})

	waiter, err := drv.docker.AttachToContainerNonBlocking(ctx, docker.AttachToContainerOptions{
		Container:    container,
		InputStream:  task.Input(),
		OutputStream: mwOut,
		ErrorStream:  mwErr,
		Success:      successChan,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
		Stdin:        true,
	})

	if err == nil {
		mon := make(chan struct{})

		// We block here, since we would like to have stdin/stdout/stderr
		// streams already attached before starting task and I/O.
		// if AttachToContainerNonBlocking() returns no error, then we'll
		// sync up with NB Attacher above before starting the task. However,
		// we might leak our go-routine if AttachToContainerNonBlocking()
		// Dial/HTTP does not honor the Success channel contract.
		// Here we assume that if our context times out, then underlying
		// go-routines in AttachToContainerNonBlocking() will unlock
		// (or eventually timeout) once we tear down the container.
		go func() {
			<-successChan
			successChan <- struct{}{}
			close(mon)
		}()

		select {
		case <-ctx.Done():
		case <-mon:
		}
	}

	if err != nil && ctx.Err() == nil {
		// ignore if ctx has errored, rewrite status lay below
		return nil, err
	}

	// we want to stop trying to collect stats when the container exits
	// collectStats will stop when stopSignal is closed or ctx is cancelled
	stopSignal := make(chan struct{})
	go drv.collectStats(ctx, stopSignal, container, task)

	err = drv.startTask(ctx, container)
	if err != nil && ctx.Err() == nil {
		// if there's just a timeout making the docker calls, drv.wait below will rewrite it to timeout
		return nil, err
	}

	return &waitResult{
		container: container,
		waiter:    waiter,
		drv:       drv,
		done:      stopSignal,
	}, nil
}

// waitResult implements drivers.WaitResult
type waitResult struct {
	container string
	waiter    docker.CloseWaiter
	drv       *DockerDriver
	done      chan struct{}
}

// waitResult implements drivers.WaitResult
func (w *waitResult) Wait(ctx context.Context) drivers.RunResult {
	defer close(w.done)

	// wait until container is stopped (or ctx is cancelled if sooner)
	status, err := w.wait(ctx)
	return &runResult{
		status: status,
		err:    err,
	}
}

// Repeatedly collect stats from the specified docker container until the stopSignal is closed or the context is cancelled
func (drv *DockerDriver) collectStats(ctx context.Context, stopSignal <-chan struct{}, container string, task drivers.ContainerTask) {
	ctx, span := trace.StartSpan(ctx, "docker_collect_stats")
	defer span.End()

	log := common.Logger(ctx)

	// dockerCallDone is used to cancel the call to drv.docker.Stats when this method exits
	dockerCallDone := make(chan bool)
	defer close(dockerCallDone)

	dstats := make(chan *docker.Stats, 1)
	go func() {
		// NOTE: docker automatically streams every 1s. we can skip or avg samples if we'd like but
		// the memory overhead is < 1MB for 3600 stat points so this seems fine, seems better to stream
		// (internal docker api streams) than open/close stream for 1 sample over and over.
		// must be called in goroutine, docker.Stats() blocks
		err := drv.docker.Stats(docker.StatsOptions{
			ID:     container,
			Stats:  dstats,
			Stream: true,
			Done:   dockerCallDone, // A flag that enables stopping the stats operation
		})

		if err != nil && err != io.ErrClosedPipe {
			log.WithError(err).WithFields(logrus.Fields{"container": container, "call_id": task.Id()}).Error("error streaming docker stats for task")
		}
	}()

	// collect stats until context is done (i.e. until the container is terminated)
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopSignal:
			return
		case ds, ok := <-dstats:
			if !ok {
				return
			}
			stats := cherryPick(ds)
			if !time.Time(stats.Timestamp).IsZero() {
				task.WriteStat(ctx, stats)
			}
		}
	}
}

func cherryPick(ds *docker.Stats) drivers.Stat {
	// TODO cpu % is as a % of the whole system... cpu is weird since we're sharing it
	// across a bunch of containers and it scales based on how many we're sharing with,
	// do we want users to see as a % of system?
	systemDelta := float64(ds.CPUStats.SystemCPUUsage - ds.PreCPUStats.SystemCPUUsage)
	cores := float64(len(ds.CPUStats.CPUUsage.PercpuUsage))
	var cpuUser, cpuKernel, cpuTotal float64
	if systemDelta > 0 {
		// TODO we could leave these in docker format and let hud/viz tools do this instead of us... like net is, could do same for mem, too. thoughts?
		cpuUser = (float64(ds.CPUStats.CPUUsage.UsageInUsermode-ds.PreCPUStats.CPUUsage.UsageInUsermode) / systemDelta) * cores * 100.0
		cpuKernel = (float64(ds.CPUStats.CPUUsage.UsageInKernelmode-ds.PreCPUStats.CPUUsage.UsageInKernelmode) / systemDelta) * cores * 100.0
		cpuTotal = (float64(ds.CPUStats.CPUUsage.TotalUsage-ds.PreCPUStats.CPUUsage.TotalUsage) / systemDelta) * cores * 100.0
	}

	var rx, tx float64
	for _, v := range ds.Networks {
		rx += float64(v.RxBytes)
		tx += float64(v.TxBytes)
	}

	var blkRead, blkWrite uint64
	for _, bioEntry := range ds.BlkioStats.IOServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkRead = blkRead + bioEntry.Value
		case "write":
			blkWrite = blkWrite + bioEntry.Value
		}
	}

	return drivers.Stat{
		Timestamp: common.DateTime(ds.Read),
		Metrics: map[string]uint64{
			// source: https://godoc.org/github.com/fsouza/go-dockerclient#Stats
			// ex (for future expansion): {"read":"2016-08-03T18:08:05Z","pids_stats":{},"network":{},"networks":{"eth0":{"rx_bytes":508,"tx_packets":6,"rx_packets":6,"tx_bytes":508}},"memory_stats":{"stats":{"cache":16384,"pgpgout":281,"rss":8826880,"pgpgin":2440,"total_rss":8826880,"hierarchical_memory_limit":536870912,"total_pgfault":3809,"active_anon":8843264,"total_active_anon":8843264,"total_pgpgout":281,"total_cache":16384,"pgfault":3809,"total_pgpgin":2440},"max_usage":8953856,"usage":8953856,"limit":536870912},"blkio_stats":{"io_service_bytes_recursive":[{"major":202,"op":"Read"},{"major":202,"op":"Write"},{"major":202,"op":"Sync"},{"major":202,"op":"Async"},{"major":202,"op":"Total"}],"io_serviced_recursive":[{"major":202,"op":"Read"},{"major":202,"op":"Write"},{"major":202,"op":"Sync"},{"major":202,"op":"Async"},{"major":202,"op":"Total"}]},"cpu_stats":{"cpu_usage":{"percpu_usage":[47641874],"usage_in_usermode":30000000,"total_usage":47641874},"system_cpu_usage":8880800500000000,"throttling_data":{}},"precpu_stats":{"cpu_usage":{"percpu_usage":[44946186],"usage_in_usermode":30000000,"total_usage":44946186},"system_cpu_usage":8880799510000000,"throttling_data":{}}}
			// mostly stolen values from docker stats cli api...

			// net
			"net_rx": uint64(rx),
			"net_tx": uint64(tx),
			// mem
			"mem_limit": ds.MemoryStats.Limit,
			"mem_usage": ds.MemoryStats.Usage,
			// i/o
			"disk_read":  blkRead,
			"disk_write": blkWrite,
			// cpu
			"cpu_user":   uint64(cpuUser),
			"cpu_total":  uint64(cpuTotal),
			"cpu_kernel": uint64(cpuKernel),
		},
	}
}

func (drv *DockerDriver) startTask(ctx context.Context, container string) error {
	log := common.Logger(ctx)
	log.WithFields(logrus.Fields{"container": container}).Debug("Starting container execution")
	err := drv.docker.StartContainerWithContext(container, nil, ctx)
	if err != nil {
		dockerErr, ok := err.(*docker.Error)
		_, containerAlreadyRunning := err.(*docker.ContainerAlreadyRunning)
		if containerAlreadyRunning || (ok && dockerErr.Status == 304) {
			// 304=container already started -- so we can ignore error
		} else {
			return err
		}
	}
	return err
}

func (w *waitResult) wait(ctx context.Context) (status string, err error) {
	// wait retries internally until ctx is up, so we can ignore the error and
	// just say it was a timeout if we have [fatal] errors talking to docker, etc.
	// a more prevalent case is calling wait & container already finished, so again ignore err.
	exitCode, _ := w.drv.docker.WaitContainerWithContext(w.container, ctx)
	defer RecordWaitContainerResult(ctx, exitCode)

	w.waiter.Close()
	err = w.waiter.Wait()
	if err != nil {
		// plumb up i/o errors (NOTE: which MAY be typed)
		return drivers.StatusError, err
	}

	// check the context first, if it's done then exitCode is invalid iff zero
	// (can't know 100% without inspecting, but that's expensive and this is a good guess)
	// if exitCode is non-zero, we prefer that since it proves termination.
	if exitCode == 0 {
		select {
		case <-ctx.Done(): // check if task was canceled or timed out
			switch ctx.Err() {
			case context.DeadlineExceeded:
				return drivers.StatusTimeout, context.DeadlineExceeded
			case context.Canceled:
				return drivers.StatusCancelled, context.Canceled
			}
		default:
		}
	}

	switch exitCode {
	default:
		return drivers.StatusError, models.NewAPIError(http.StatusBadGateway, fmt.Errorf("container exit code %d", exitCode))
	case 0:
		return drivers.StatusSuccess, nil
	case 137: // OOM
		common.Logger(ctx).Error("docker oom")
		err := errors.New("container out of memory, you may want to raise fn.memory for this function (default: 128MB)")
		return drivers.StatusKilled, models.NewAPIError(http.StatusBadGateway, err)
	}
}

var _ drivers.Driver = &DockerDriver{}

func init() {
	drivers.Register("docker", func(config drivers.Config) (drivers.Driver, error) {
		return NewDocker(config), nil
	})
}
