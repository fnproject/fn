package docker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/fnproject/fn/api/agent/drivers"
	driverstats "github.com/fnproject/fn/api/agent/drivers/stats"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
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
	DockerAuth(ctx context.Context, image string) (*types.AuthConfig, error)
}

type runResult struct {
	err    error
	status string
}

type driverAuthConfig struct {
	auth       types.AuthConfig
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
	var containers []types.Container

	for limiter.Wait(ctx) == nil {
		var err error
		ctx, cancel := context.WithTimeout(ctx, containerListTimeout)
		containers, err = driver.docker.ContainerList(ctx, types.ContainerListOptions{
			All:     true, // let's include containers that are not running, but not destroyed
			Filters: filters.NewArgs(filters.KeyValuePair{Key: "label", Value: filter}),
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

		opts := types.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}

		// If this fails, we log and continue.
		err := driver.docker.ContainerRemove(ctx, item.ID, opts)
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
		images, err := driver.docker.ImageList(ctx, types.ImageListOptions{})
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
			_, err := driver.docker.ImageRemove(ctx, img.ID, types.ImageRemoveOptions{})
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

	file, err := os.Open(filepath.Clean(driver.conf.DockerLoadFile))
	if err != nil {
		return err
	}
	defer file.Close()

	quiet := false
	resp, err := driver.docker.ImageLoad(ctx, file, quiet)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	io.Copy(&b, resp.Body)
	resp.Body.Close()

	// TODO(reed): idk that we need this/if quiet works... leaving, for science
	log.Infoln("loaded docker images: ", b.String())
	return nil
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

	opts := &container.Config{
		Image:        task.Image(),
		OpenStdin:    !stdinOff,
		StdinOnce:    !stdinOff,
		AttachStdin:  !stdinOff,
		AttachStdout: !stdoutOff,
		AttachStderr: !stderrOff,
	}
	tru := true
	hostOpts := &container.HostConfig{
		ReadonlyRootfs: drv.conf.EnableReadOnlyRootFs,
		Init:           &tru,
	}

	cookie := &cookie{
		opts:     opts,
		hostOpts: hostOpts,
		task:     task,
		drv:      drv,
	}

	// Order is important, eg. Hostname doesn't play well with Network config
	cookie.configureLabels(log)
	cookie.configureLogger(log)
	cookie.configureMem(log)
	cookie.configureCmd(log)
	cookie.configureEnv(log)
	cookie.configureCPU(log)
	cookie.configureFsSize(log)
	cookie.configurePIDs(log)
	cookie.configureULimits(log)
	cookie.configureTmpFs(log)
	cookie.configureVolumes(log)
	cookie.configureWorkDir(log)
	cookie.configureIOFS(log)
	cookie.configureNetwork(log)
	cookie.configureHostname(log)
	cookie.configureSecurity(log)

	return cookie, nil
}

// Run executes the docker container. If task runs, drivers.RunResult will be returned. If something fails outside the task (ie: Docker), it will return error.
// The docker driver will attempt to cast the task to a Auther. If that succeeds, private image support is available. See the Auther interface for how to implement this.
func (drv *DockerDriver) run(ctx context.Context, task drivers.ContainerTask) (drivers.WaitResult, error) {
	container := task.Id()

	log := common.Logger(ctx)
	stdin := task.Input()
	stdout, stderr := task.Logger()
	successChan := make(chan struct{})

	_, stdinOff := stdin.(common.NoopReadWriteCloser)
	_, stdoutOff := stdout.(common.NoopReadWriteCloser)
	_, stderrOff := stderr.(common.NoopReadWriteCloser)

	resp, errAttach := drv.docker.ContainerAttach(ctx, container, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  !stdinOff,
		Stdout: !stdoutOff,
		Stderr: !stderrOff,
	})
	if errAttach != nil && errAttach != httputil.ErrPersistEOF {
		// ContainerAttach return an ErrPersistEOF (connection closed)
		// means server met an error and already put it in Hijacked connection,
		// we would keep the error and read the detailed error message from hijacked connection
		log.WithError(errAttach).WithFields(logrus.Fields{"container": container, "call_id": task.Id()}).Error("error attaching to container")
		return nil, errAttach
	}

	// TODO write stdin / read stdout/stderr?
	// where to put dis... hmmm.

	ctx, cancel := context.WithCancel(ctx)
	go drv.collectStats(ctx, task)

	cancel = func() {
		resp.Close()
		cancel()
	}

	cErr := make(chan error, 1)

	go func() {
		cErr <- func() error {
			streamer := hijackedIOStreamer{
				streams:      dockerCli,
				inputStream:  in,
				outputStream: dockerCli.Out(),
				errorStream:  dockerCli.Err(),
				resp:         resp,
				tty:          c.Config.Tty,
				detachKeys:   options.DetachKeys,
			}

			errHijack := streamer.stream(ctx)
			if errHijack == nil {
				// XXX(reed): idk why docker cli carries this error to here, think about it
				return errAttach
			}
			return errHijack
		}()
	}()

	err = drv.docker.ContainerStart(ctx, container, types.ContainerStartOptions{})
	if err != nil {
		cancel() // make sure we shut down stats / attach & close body
		<-cErr

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
		errCh:     cErr,
		drv:       drv,
		done:      cancel,
	}, nil
}

// XXX(reed): we do not need most of hijackedIOStreamer, gut most of this

// A hijackedIOStreamer handles copying input to and output from streams to the
// connection.
type hijackedIOStreamer struct {
	streams      command.Streams
	inputStream  io.ReadCloser
	outputStream io.Writer
	errorStream  io.Writer

	resp types.HijackedResponse

	tty        bool
	detachKeys string
}

// stream handles setting up the IO and then begins streaming stdin/stdout
// to/from the hijacked connection, blocking until it is either done reading
// output, the user inputs the detach key sequence when in TTY mode, or when
// the given context is cancelled.
func (h *hijackedIOStreamer) stream(ctx context.Context) error {
	restoreInput, err := h.setupInput()
	if err != nil {
		return fmt.Errorf("unable to setup input stream: %s", err)
	}

	defer restoreInput()

	outputDone := h.beginOutputStream(restoreInput)
	inputDone, detached := h.beginInputStream(restoreInput)

	select {
	case err := <-outputDone:
		return err
	case <-inputDone:
		// Input stream has closed.
		if h.outputStream != nil || h.errorStream != nil {
			// Wait for output to complete streaming.
			select {
			case err := <-outputDone:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	case err := <-detached:
		// Got a detach key sequence.
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *hijackedIOStreamer) setupInput() (restore func(), err error) {
	if h.inputStream == nil || !h.tty {
		// No need to setup input TTY.
		// The restore func is a nop.
		return func() {}, nil
	}

	if err := setRawTerminal(h.streams); err != nil {
		return nil, fmt.Errorf("unable to set IO streams as raw terminal: %s", err)
	}

	// Use sync.Once so we may call restore multiple times but ensure we
	// only restore the terminal once.
	var restoreOnce sync.Once
	restore = func() {
		restoreOnce.Do(func() {
			restoreTerminal(h.streams, h.inputStream)
		})
	}

	// Wrap the input to detect detach escape sequence.
	// Use default escape keys if an invalid sequence is given.
	escapeKeys := defaultEscapeKeys
	if h.detachKeys != "" {
		customEscapeKeys, err := term.ToBytes(h.detachKeys)
		if err != nil {
			logrus.Warnf("invalid detach escape keys, using default: %s", err)
		} else {
			escapeKeys = customEscapeKeys
		}
	}

	h.inputStream = ioutils.NewReadCloserWrapper(term.NewEscapeProxy(h.inputStream, escapeKeys), h.inputStream.Close)

	return restore, nil
}

func (h *hijackedIOStreamer) beginOutputStream(restoreInput func()) <-chan error {
	if h.outputStream == nil && h.errorStream == nil {
		// There is no need to copy output.
		return nil
	}

	outputDone := make(chan error)
	go func() {
		var err error

		// When TTY is ON, use regular copy
		if h.outputStream != nil && h.tty {
			_, err = io.Copy(h.outputStream, h.resp.Reader)
			// We should restore the terminal as soon as possible
			// once the connection ends so any following print
			// messages will be in normal type.
			restoreInput()
		} else {
			_, err = stdcopy.StdCopy(h.outputStream, h.errorStream, h.resp.Reader)
		}

		logrus.Debug("[hijack] End of stdout")

		if err != nil {
			logrus.Debugf("Error receiveStdout: %s", err)
		}

		outputDone <- err
	}()

	return outputDone
}

func (h *hijackedIOStreamer) beginInputStream(restoreInput func()) (doneC <-chan struct{}, detachedC <-chan error) {
	inputDone := make(chan struct{})
	detached := make(chan error)

	go func() {
		if h.inputStream != nil {
			_, err := io.Copy(h.resp.Conn, h.inputStream)
			// We should restore the terminal as soon as possible
			// once the connection ends so any following print
			// messages will be in normal type.
			restoreInput()

			logrus.Debug("[hijack] End of stdin")

			if _, ok := err.(term.EscapeError); ok {
				detached <- err
				return
			}

			if err != nil {
				// This error will also occur on the receive
				// side (from stdout) where it will be
				// propagated back to the caller.
				logrus.Debugf("Error sendStdin: %s", err)
			}
		}

		if err := h.resp.CloseWrite(); err != nil {
			logrus.Debugf("Couldn't send EOF: %s", err)
		}

		close(inputDone)
	}()

	return inputDone, detached
}

func setRawTerminal(streams command.Streams) error {
	if err := streams.In().SetRawTerminal(); err != nil {
		return err
	}
	return streams.Out().SetRawTerminal()
}

// nolint: unparam
func restoreTerminal(streams command.Streams, in io.Closer) error {
	streams.In().RestoreTerminal()
	streams.Out().RestoreTerminal()
	// WARNING: DO NOT REMOVE THE OS CHECKS !!!
	// For some reason this Close call blocks on darwin..
	// As the client exits right after, simply discard the close
	// until we find a better solution.
	//
	// This can also cause the client on Windows to get stuck in Win32 CloseHandle()
	// in some cases. See https://github.com/docker/docker/issues/28267#issuecomment-288237442
	// Tracked internally at Microsoft by VSO #11352156. In the
	// Windows case, you hit this if you are using the native/v2 console,
	// not the "legacy" console, and you start the client in a new window. eg
	// `start docker run --rm -it microsoft/nanoserver cmd /s /c echo foobar`
	// will hang. Remove start, and it won't repro.
	if in != nil && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return in.Close()
	}
	return nil
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
	errCh     <-chan error
	drv       *DockerDriver
	done      func() // cancel for stats
}

// waitResult implements drivers.WaitResult
func (w *waitResult) Wait(ctx context.Context) drivers.RunResult {
	// wait until container is stopped (or ctx is cancelled if sooner)
	status, err := w.wait(ctx)
	return &runResult{
		status: status,
		err:    err,
	}
}

// Repeatedly collect stats from the specified docker container until the context is cancelled
func (drv *DockerDriver) collectStats(ctx context.Context, task drivers.ContainerTask) {
	ctx, span := trace.StartSpan(ctx, "docker_collect_stats")
	defer span.End()

	container := task.Id()
	log := common.Logger(ctx)

	// NOTE: docker only streams every 1s. beware of load on docker when not using streaming.
	stream := true
	resp, err := drv.docker.ContainerStats(ctx, container, stream)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"container": container}).Error("error streaming docker stats for task")
		return
	}
	defer resp.Body.Close()

	// docker only does every 1s. ticker is supposed to adjust to missed ticks, should be ok here
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			dec := json.NewDecoder(resp.Body)
			var stat types.StatsJSON
			dec.Decode(&stat)

			if !time.Time(stat.Read).IsZero() {
				stats := cherryPick(stat)
				task.WriteStat(ctx, stats)
			}
		}
	}
}

func cherryPick(ds types.StatsJSON) driverstats.Stat {
	// TODO cpu % is as a % of the whole system... cpu is weird since we're sharing it
	// across a bunch of containers and it scales based on how many we're sharing with,
	// do we want users to see as a % of system?
	systemDelta := float64(ds.CPUStats.SystemUsage - ds.PreCPUStats.SystemUsage)
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
	for _, bioEntry := range ds.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkRead = blkRead + bioEntry.Value
		case "write":
			blkWrite = blkWrite + bioEntry.Value
		}
	}

	return driverstats.Stat{
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

func recordWaitContainerResult(ctx context.Context, exitCode int64) {

	// Tag the metric with error-code or context-cancel/deadline info
	exitStr := fmt.Sprintf("exit_%d", exitCode)
	if exitCode == 0 && ctx.Err() != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			exitStr = "ctx_deadline"
		case context.Canceled:
			exitStr = "ctx_canceled"
		}
	}

	ctx, err := tag.New(ctx,
		tag.Upsert(apiNameKey, "docker_wait_container"),
		tag.Upsert(exitStatusKey, exitStr),
	)
	if err != nil {
		logrus.WithError(err).Fatalf("cannot add tag %v=%v or tag %v=docker_wait_container", exitStatusKey, exitStr, apiNameKey)
	}
	stats.Record(ctx, dockerExitMeasure.M(0))
}

func (w *waitResult) wait(ctx context.Context) (status string, err error) {
	// TODO(reed): should this be WaitConditionNextExit? (see CLI) contract is weird, we don't expect restarts?
	ch, waitErr := w.drv.docker.ContainerWait(ctx, w.container, containertypes.WaitConditionNotRunning)

	var exitCode int64
	select {
	case wait := <-ch:
		exitCode = wait.StatusCode
		if wait.Error != nil && wait.Error.Message != "" {
			// TODO(reed): docker cli rewrites to 125 here
			err = errors.New(wait.Error.Message)
		}
	case err = <-waitErr:
		// TODO(reed): docker cli rewrites to 125 here
	}

	if err != nil {
		log := common.Logger(ctx)
		log.WithError(err).WithFields(logrus.Fields{"container": w.container}).Error("error waiting container")
	}

	defer recordWaitContainerResult(ctx, exitCode)

	// this closes attach and stats
	// NOTE: this MUST get called if waitResult.Wait() is called
	w.done()

	// this gets the error out of attach
	err = <-w.errCh
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
