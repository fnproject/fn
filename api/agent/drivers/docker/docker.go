package docker

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fnproject/fn/api/agent/drivers/stats"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/coreos/go-semver/semver"
	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
	"golang.org/x/time/rate"
)

const (
	FnAgentClassifierLabel = "fn-agent-classifier"
	FnAgentInstanceLabel   = "fn-agent-instance"
)

// Auther may by implemented by a drivers.ContainerTask if it would
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
	DockerAuth(ctx context.Context, image string) (*docker.AuthConfiguration, error)
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

// DockerDriver implements drivers.Driver via the docker http API
type DockerDriver struct {
	cancel   func()
	conf     drivers.Config
	docker   dockerClient // retries on *docker.Client, restricts ad hoc *docker.Client usage / retries
	hostname string
	auths    map[string]driverAuthConfig
	pool     DockerPool
	network  *DockerNetworks

	instanceId string

	imgCache  ImageCacher
	imgPuller ImagePuller
}

// NewDocker implements drivers.Driver
func NewDocker(conf drivers.Config) *DockerDriver {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't resolve hostname")
	}

	// This is for testing purposes. Tests override with custom id
	instanceId := conf.InstanceId
	if instanceId == "" {
		instanceId, err = generateRandUUID()
		if err != nil {
			logrus.WithError(err).Fatal("couldn't initialize instanceId")
		}
	}

	auths, err := registryFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't initialize registry")
	}

	ctx, cancel := context.WithCancel(context.Background())
	driver := &DockerDriver{
		cancel:     cancel,
		conf:       conf,
		docker:     newClient(ctx),
		hostname:   hostname,
		auths:      auths,
		network:    NewDockerNetworks(conf),
		instanceId: instanceId,
		imgCache:   createImageCache(conf),
	}

	err = checkDockerVersion(ctx, driver)
	if err != nil {
		logrus.WithError(err).Fatal("docker version error")
	}

	// start the cleanup jobs as early as possible
	go func() {
		killLeakedContainers(ctx, driver)
		runImageStats(ctx, driver)
		syncImageCleaner(ctx, driver)
		runImageCleaner(ctx, driver)
	}()

	// before we do anything else, let's pre-load requested images
	err = loadDockerImages(ctx, driver)
	if err != nil {
		logrus.WithError(err).Fatalf("cannot load docker images in %s", conf.DockerLoadFile)
	}

	driver.imgPuller = NewImagePuller(driver.docker)

	// finally spawn pool if enabled
	if conf.PreForkPoolSize != 0 {
		driver.pool = NewDockerPool(conf, driver)
	}

	// Record our instance id at startup
	RecordInstanceId(ctx, instanceId)

	return driver
}

// createImageCache scans the driver config to spawn an image cacher if applicable
func createImageCache(conf drivers.Config) ImageCacher {
	if conf.ImageCleanMaxSize == 0 {
		return nil
	}

	exemptImages := strings.Fields(conf.ImageCleanExemptTags)
	// we never want to remove prefork image
	if conf.PreForkPoolSize != 0 && conf.PreForkImage != "" {
		exemptImages = append(exemptImages, conf.PreForkImage)
	}

	// WARNING: assuming images in conf.DockerLoadFile are also added in conf.ImageCleanExemptTags
	return NewImageCache(exemptImages, conf.ImageCleanMaxSize)
}

// killLeakedContainers scans and destroys previously left over containers that were managed
// by this docker driver. This operation is executed once and if it fails, it will not
// retry the procedure.
func killLeakedContainers(ctx context.Context, driver *DockerDriver) {

	// Label Tag is used to isolate this cleanup. If docker has other containers
	// that are not managed by fn-agent, then this tag can make sure those containers
	// are not killed. For this reason, we require this tag to be set.
	if driver.conf.ContainerLabelTag == "" {
		return
	}

	const containerListTimeout = time.Duration(60 * time.Second)

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "killLeakedContainers"})
	limiter := rate.NewLimiter(2.0, 1)

	filter := fmt.Sprintf("%s=%s", FnAgentClassifierLabel, driver.conf.ContainerLabelTag)
	var containers []docker.APIContainers

	for limiter.Wait(ctx) == nil {
		var err error
		ctx, cancel := context.WithTimeout(ctx, containerListTimeout)
		containers, err = driver.docker.ListContainers(docker.ListContainersOptions{
			All: true, // let's include containers that are not running, but not destroyed
			Filters: map[string][]string{
				"label": []string{filter},
			},
			Context: ctx,
		})
		cancel()
		if err == nil {
			break
		}

		log.WithError(err).Error("ListContainers error, will retry...")
	}

	for _, item := range containers {
		logrus.Debugf("checking %+v", item)

		// skip containers that belong to our current running agent/docker instance
		if item.Labels[FnAgentInstanceLabel] == driver.instanceId {
			continue
		}

		logger := logrus.WithFields(logrus.Fields{"container_id": item.ID, "image": item.Image, "state": item.State})
		logger.Info("Terminating dangling docker container")

		opts := docker.RemoveContainerOptions{
			ID:            item.ID,
			Force:         true,
			RemoveVolumes: true,
			Context:       ctx,
		}

		// If this fails, we log and continue.
		err := driver.docker.RemoveContainer(opts)
		if err != nil {
			logger.WithError(err).Error("cannot remove container")
		}
	}
}

// syncImageCleaner lists the current images on the system and adds them to the
// image cache. The operation is performed once during startup to ensure a
// restart of the fn-agent keeps track of previous state.
func syncImageCleaner(ctx context.Context, driver *DockerDriver) {
	if driver.imgCache == nil {
		return
	}

	const imageListTimeout = time.Duration(60 * time.Second)

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "syncImageCleaner"})
	limiter := rate.NewLimiter(2.0, 1)

	for limiter.Wait(ctx) == nil {
		ctx, cancel := context.WithTimeout(ctx, imageListTimeout)
		images, err := driver.docker.ListImages(docker.ListImagesOptions{Context: ctx})
		cancel()

		if err == nil {
			for _, img := range images {
				driver.imgCache.Update(&CachedImage{
					ID:       img.ID,
					ParentID: img.ParentID,
					RepoTags: img.RepoTags,
					Size:     uint64(img.Size),
				})
			}
			return
		}

		log.WithError(err).Error("ListImages error, will retry...")
	}
}

// runImageStats runs continuously in background to periodically sample image cleaner statistics
func runImageStats(ctx context.Context, driver *DockerDriver) {
	if driver.imgCache == nil {
		return
	}

	const statsSamplerDuration = time.Duration(2 * time.Second)
	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"stack": "runImageStats"})

	go func() {
		ticker := time.NewTicker(statsSamplerDuration)
		defer ticker.Stop()

		for ctx.Err() == nil {
			RecordImageCleanerStats(ctx, driver.imgCache.GetStats())
			select {
			case <-ctx.Done(): // driver shutdown
			case <-ticker.C:
			}
		}
	}()
}

// runImageCleaner runs continuously and monitors image cache state. If the
// cache is over the high water mark limit, then it tries to remove least recently
// used image.
func runImageCleaner(ctx context.Context, driver *DockerDriver) {
	if driver.imgCache == nil {
		return
	}

	const removeImgTimeout = time.Duration(60 * time.Second)

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "runImageCleaner"})
	limiter := rate.NewLimiter(2.0, 1)
	notifier := driver.imgCache.GetNotifier()

	for limiter.Wait(ctx) == nil {
		for !driver.imgCache.IsMaxCapacity() {
			select {
			case <-ctx.Done(): // driver shutdown
				return
			case <-notifier:
			}
		}

		img := driver.imgCache.Pop()
		if img != nil {
			log.WithField("image", img).Info("Removing image")

			ctx, cancel := context.WithTimeout(ctx, removeImgTimeout)
			err := driver.docker.RemoveImage(img.ID, docker.RemoveImageOptions{Context: ctx})
			cancel()
			if err != nil && err != docker.ErrNoSuchImage {
				log.WithError(err).WithField("image", img).Error("Removing image failed")
				// in-use or can't be removed or docker just timed out, try to add it back to the cache
				driver.imgCache.Update(img)
			}
		}
	}
}

func checkDockerVersion(ctx context.Context, driver *DockerDriver) error {
	if driver.conf.ServerVersion == "" {
		return nil
	}

	info, err := driver.docker.Info(ctx)
	if err != nil {
		return err
	}

	actual, err := semver.NewVersion(info.ServerVersion)
	if err != nil {
		return err
	}

	wanted, err := semver.NewVersion(driver.conf.ServerVersion)
	if err != nil {
		return err
	}

	if actual.Compare(*wanted) < 0 {
		return fmt.Errorf("docker version is too old. Required: %s Found: %s", driver.conf.ServerVersion, info.ServerVersion)
	}

	return nil
}

func loadDockerImages(ctx context.Context, driver *DockerDriver) error {
	if driver.conf.DockerLoadFile == "" {
		return nil
	}

	var log logrus.FieldLogger
	ctx, log = common.LoggerWithFields(ctx, logrus.Fields{"stack": "loadDockerImages"})
	log.Infof("Loading docker images from %v", driver.conf.DockerLoadFile)
	return driver.docker.LoadImages(ctx, driver.conf.DockerLoadFile)
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

func (drv *DockerDriver) SetPullImageRetryPolicy(policy common.BackOffConfig, checker drivers.RetryErrorChecker) error {
	return drv.imgPuller.SetRetryPolicy(policy, checker)
}

func (drv *DockerDriver) CreateCookie(ctx context.Context, task drivers.ContainerTask) (drivers.Cookie, error) {

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "CreateCookie"})

	_, stdinOff := task.Input().(common.NoopReadWriteCloser)
	stdout, stderr := task.Logger()
	_, stdoutOff := stdout.(common.NoopReadWriteCloser)
	_, stderrOff := stderr.(common.NoopReadWriteCloser)

	opts := docker.CreateContainerOptions{
		Name: task.Id(),
		Config: &docker.Config{
			Image:        task.Image(),
			OpenStdin:    !stdinOff,
			StdinOnce:    !stdinOff,
			AttachStdin:  !stdinOff,
			AttachStdout: !stdoutOff,
			AttachStderr: !stderrOff,
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

	// Order is important, eg. Hostname doesn't play well with Network config
	cookie.configureLabels(log)
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
	cookie.configureNetwork(log)
	cookie.configureHostname(log)
	cookie.configureImage(log)

	return cookie, nil
}

// Run executes the docker container. If task runs, drivers.RunResult will be returned. If something fails outside the task (ie: Docker), it will return error.
// The docker driver will attempt to cast the task to a Auther. If that succeeds, private image support is available. See the Auther interface for how to implement this.
func (drv *DockerDriver) run(ctx context.Context, container string, task drivers.ContainerTask) (drivers.WaitResult, error) {

	log := common.Logger(ctx)
	stdout, stderr := task.Logger()
	successChan := make(chan struct{})

	_, stdinOff := task.Input().(common.NoopReadWriteCloser)
	_, stdoutOff := stdout.(common.NoopReadWriteCloser)
	_, stderrOff := stderr.(common.NoopReadWriteCloser)

	waiter, err := drv.docker.AttachToContainerNonBlocking(ctx, docker.AttachToContainerOptions{
		Container:    container,
		InputStream:  task.Input(),
		OutputStream: stdout,
		ErrorStream:  stderr,
		Success:      successChan,
		Stream:       true,
		Stdout:       !stdoutOff,
		Stderr:       !stderrOff,
		Stdin:        !stdinOff,
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
		log.WithError(err).WithFields(logrus.Fields{"container": container, "call_id": task.Id()}).Error("error attaching to container")
		return nil, err
	}

	// we want to stop trying to collect stats when the container exits
	// collectStats will stop when stopSignal is closed or ctx is cancelled
	stopSignal := make(chan struct{})
	go drv.collectStats(ctx, stopSignal, container, task)

	err = drv.docker.StartContainerWithContext(container, nil, ctx)
	if err != nil && ctx.Err() == nil {
		if isSyslogError(err) {
			// syslog error is a func error
			e := models.NewAPIError(http.StatusInternalServerError, errors.New("Syslog Unavailable"))
			return nil, models.NewFuncError(e)
		}
		// if there's just a timeout making the docker calls, drv.wait below will rewrite it to timeout
		log.WithError(err).WithFields(logrus.Fields{"container": container, "call_id": task.Id()}).Error("error starting container")
		return nil, err
	}

	return &waitResult{
		container: container,
		waiter:    waiter,
		drv:       drv,
		done:      stopSignal,
	}, nil
}

// isSyslogError checks if the error message is what docker syslog plugin returns
// when not able to connect to syslog
func isSyslogError(err error) bool {
	derr, ok := err.(*docker.Error)
	return ok && strings.HasPrefix(derr.Message, "failed to initialize logging driver")
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
			ID:      container,
			Stats:   dstats,
			Stream:  true,
			Done:    dockerCallDone, // A flag that enables stopping the stats operation
			Context: common.BackgroundContext(ctx),
		})

		if err != nil {
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

func cherryPick(ds *docker.Stats) stats.Stat {
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

	return stats.Stat{
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

func (w *waitResult) wait(ctx context.Context) (status string, err error) {
	exitCode, waitErr := w.drv.docker.WaitContainerWithContext(w.container, ctx)
	if waitErr != nil {
		log := common.Logger(ctx)
		log.WithError(waitErr).WithFields(logrus.Fields{"container": w.container}).Error("error waiting container with context")
	}

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

func generateRandUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	uuid := fmt.Sprintf("%x%x%x%x%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	return uuid, nil
}
