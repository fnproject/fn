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

	"go.opencensus.io/stats"
	"go.opencensus.io/trace"

	"github.com/coreos/go-semver/semver"
	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/fsouza/go-dockerclient"
	"github.com/go-openapi/strfmt"
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
	DockerAuth() (docker.AuthConfiguration, error)
}

type runResult struct {
	err    error
	status string
}

func (r *runResult) Error() error   { return r.err }
func (r *runResult) Status() string { return r.status }

type DockerDriver struct {
	conf     drivers.Config
	docker   dockerClient // retries on *docker.Client, restricts ad hoc *docker.Client usage / retries
	hostname string
	auths    map[string]docker.AuthConfiguration
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

	driver := &DockerDriver{
		conf:     conf,
		docker:   newClient(),
		hostname: hostname,
		auths:    registryFromEnv(),
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

func registryFromEnv() map[string]docker.AuthConfiguration {
	var auths *docker.AuthConfigurations
	var err error
	if reg := os.Getenv("DOCKER_AUTH"); reg != "" {
		// TODO docker does not use this itself, we should get rid of env docker config (nor is this documented..)
		auths, err = docker.NewAuthConfigurations(strings.NewReader(reg))
	} else {
		auths, err = docker.NewAuthConfigurationsFromDockerCfg()
	}

	if err != nil {
		logrus.WithError(err).Info("no docker auths from config files found (this is fine)")
		return nil
	}
	return auths.Configs
}

func (drv *DockerDriver) Close() error {
	var err error
	if drv.pool != nil {
		err = drv.pool.Close()
	}
	return err
}

func (drv *DockerDriver) pickPool(ctx context.Context, container *docker.CreateContainerOptions) string {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "tryUsePool"})

	if drv.pool == nil || container.HostConfig.NetworkMode != "" {
		return ""
	}

	id, err := drv.pool.AllocPoolId()
	if err != nil {
		log.WithError(err).Error("Could not fetch pre fork pool container")
		return ""
	}

	// We are able to fetch a container from pool. Now, use its
	// network, ipc and pid namespaces.
	container.HostConfig.NetworkMode = fmt.Sprintf("container:%s", id)
	//container.HostConfig.IpcMode = linker
	//container.HostConfig.PidMode = linker
	return id
}

func (drv *DockerDriver) unpickPool(poolId string) {
	if poolId != "" && drv.pool != nil {
		drv.pool.FreePoolId(poolId)
	}
}

func (drv *DockerDriver) pickNetwork(container *docker.CreateContainerOptions) string {

	if len(drv.networks) == 0 || container.HostConfig.NetworkMode != "" {
		return ""
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

	container.HostConfig.NetworkMode = id
	return id
}

func (drv *DockerDriver) unpickNetwork(netId string) {
	if netId != "" {
		drv.networksLock.Lock()
		drv.networks[netId]--
		drv.networksLock.Unlock()
	}
}

func (drv *DockerDriver) configureFs(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	if task.FsSize() != 0 {
		// If defined, impose file system size limit. In MB units.
		if container.HostConfig.StorageOpt == nil {
			container.HostConfig.StorageOpt = make(map[string]string)
		}

		opt := fmt.Sprintf("%vM", task.FsSize())

		log.WithFields(logrus.Fields{"size": opt, "call_id": task.Id()}).Debug("setting storage option")
		container.HostConfig.StorageOpt["size"] = opt
	}

	container.HostConfig.ReadonlyRootfs = drv.conf.EnableReadOnlyRootFs
}

func (drv *DockerDriver) configureTmpFs(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	if task.TmpFsSize() == 0 && !drv.conf.EnableReadOnlyRootFs {
		return
	}

	if container.HostConfig.Tmpfs == nil {
		container.HostConfig.Tmpfs = make(map[string]string)
	}

	var tmpFsOption string
	if task.TmpFsSize() != 0 {
		if drv.conf.MaxTmpFsInodes != 0 {
			tmpFsOption = fmt.Sprintf("size=%dm,nr_inodes=%d", task.TmpFsSize(), drv.conf.MaxTmpFsInodes)
		} else {
			tmpFsOption = fmt.Sprintf("size=%dm", task.TmpFsSize())
		}
	}
	target := "/tmp"

	log.WithFields(logrus.Fields{"target": target, "options": tmpFsOption, "call_id": task.Id()}).Debug("setting tmpfs")
	container.HostConfig.Tmpfs[target] = tmpFsOption
}

func (drv *DockerDriver) configureVolumes(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	if len(task.Volumes()) == 0 {
		return
	}

	if container.Config.Volumes == nil {
		container.Config.Volumes = map[string]struct{}{}
	}

	for _, mapping := range task.Volumes() {
		hostDir := mapping[0]
		containerDir := mapping[1]
		container.Config.Volumes[containerDir] = struct{}{}
		mapn := fmt.Sprintf("%s:%s", hostDir, containerDir)
		container.HostConfig.Binds = append(container.HostConfig.Binds, mapn)
		log.WithFields(logrus.Fields{"volumes": mapn, "call_id": task.Id()}).Debug("setting volumes")
	}
}

func (drv *DockerDriver) configureCPU(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	// Translate milli cpus into CPUQuota & CPUPeriod (see Linux cGroups CFS cgroup v1 documentation)
	// eg: task.CPUQuota() of 8000 means CPUQuota of 8 * 100000 usecs in 100000 usec period,
	// which is approx 8 CPUS in CFS world.
	// Also see docker run options --cpu-quota and --cpu-period
	if task.CPUs() == 0 {
		return
	}

	quota := int64(task.CPUs() * 100)
	period := int64(100000)

	log.WithFields(logrus.Fields{"quota": quota, "period": period, "call_id": task.Id()}).Debug("setting CPU")
	container.HostConfig.CPUQuota = quota
	container.HostConfig.CPUPeriod = period
}

func (drv *DockerDriver) configureWorkDir(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	if wd := task.WorkDir(); wd != "" {
		log.WithFields(logrus.Fields{"wd": wd, "call_id": task.Id()}).Debug("setting work dir")
		container.Config.WorkingDir = wd
	}
}

func (drv *DockerDriver) configureHostname(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	if container.HostConfig.NetworkMode == "" {
		// hostname and container NetworkMode is not compatible.
		log.WithFields(logrus.Fields{"hostname": drv.hostname, "call_id": task.Id()}).Debug("setting hostname")
		container.Config.Hostname = drv.hostname
	}
}

func (drv *DockerDriver) configureCmd(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	if task.Command() == "" {
		return
	}

	// NOTE: this is hyper-sensitive and may not be correct like this even, but it passes old tests
	cmd := strings.Fields(task.Command())
	log.WithFields(logrus.Fields{"call_id": task.Id(), "cmd": cmd, "len": len(cmd)}).Debug("docker command")
	container.Config.Cmd = cmd
}

func (drv *DockerDriver) configureEnv(log logrus.FieldLogger, container *docker.CreateContainerOptions, task drivers.ContainerTask) {
	envvars := make([]string, 0, len(task.EnvVars()))
	for name, val := range task.EnvVars() {
		envvars = append(envvars, name+"="+val)
	}

	container.Config.Env = envvars
}

func (drv *DockerDriver) Prepare(ctx context.Context, task drivers.ContainerTask) (drivers.Cookie, error) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "Prepare"})

	container := docker.CreateContainerOptions{
		Name: task.Id(),
		Config: &docker.Config{
			Memory:       int64(task.Memory()),
			MemorySwap:   int64(task.Memory()), // disables swap
			KernelMemory: int64(task.Memory()),
			Image:        task.Image(),
			OpenStdin:    true,
			AttachStdout: true,
			AttachStderr: true,
			AttachStdin:  true,
			StdinOnce:    true,
		},
		// turn off logs since we're collecting them from attach
		HostConfig: &docker.HostConfig{
			LogConfig: docker.LogConfig{
				Type: "none",
			},
		},
		Context: ctx,
	}

	drv.configureCmd(log, &container, task)
	drv.configureEnv(log, &container, task)
	drv.configureCPU(log, &container, task)
	drv.configureFs(log, &container, task)
	drv.configureTmpFs(log, &container, task)
	drv.configureVolumes(log, &container, task)
	drv.configureWorkDir(log, &container, task)

	poolId := drv.pickPool(ctx, &container)
	netId := drv.pickNetwork(&container)

	drv.configureHostname(log, &container, task)

	err := drv.ensureImage(ctx, task)
	if err != nil {
		drv.unpickPool(poolId)
		drv.unpickNetwork(netId)
		return nil, err
	}

	_, err = drv.docker.CreateContainer(container)
	if err != nil {
		// since we retry under the hood, if the container gets created and retry fails, we can just ignore error
		if err != docker.ErrContainerAlreadyExists {
			log.WithFields(logrus.Fields{"call_id": task.Id(), "command": container.Config.Cmd, "memory": container.Config.Memory,
				"cpu_quota": task.CPUs(), "hostname": container.Config.Hostname, "name": container.Name,
				"image": container.Config.Image, "volumes": container.Config.Volumes, "binds": container.HostConfig.Binds, "container": container.Name,
			}).WithError(err).Error("Could not create container")

			// NOTE: if the container fails to create we don't really want to show to user since they aren't directly configuring the container
			drv.unpickPool(poolId)
			drv.unpickNetwork(netId)
			return nil, err
		}
	}

	// discard removal error
	return &cookie{id: task.Id(), task: task, drv: drv, poolId: poolId, netId: netId}, nil
}

type cookie struct {
	id     string
	poolId string
	netId  string
	task   drivers.ContainerTask
	drv    *DockerDriver
}

func (c *cookie) Close(ctx context.Context) error {
	defer func() {
		c.drv.unpickPool(c.poolId)
		c.drv.unpickNetwork(c.netId)
	}()

	return c.drv.removeContainer(ctx, c.id)
}

func (c *cookie) Run(ctx context.Context) (drivers.WaitResult, error) {
	return c.drv.run(ctx, c.id, c.task)
}

func (c *cookie) Freeze(ctx context.Context) error {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "Freeze"})
	log.WithFields(logrus.Fields{"call_id": c.id}).Debug("docker pause")

	err := c.drv.docker.PauseContainer(c.id, ctx)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"call_id": c.id}).Error("error pausing container")
	}
	return err
}

func (c *cookie) Unfreeze(ctx context.Context) error {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "Unfreeze"})
	log.WithFields(logrus.Fields{"call_id": c.id}).Debug("docker unpause")

	err := c.drv.docker.UnpauseContainer(c.id, ctx)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"call_id": c.id}).Error("error unpausing container")
	}
	return err
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
	reg, _, _ := drivers.ParseImage(task.Image())

	// ask for docker creds before looking for image, as the tasker may need to
	// validate creds even if the image is downloaded.

	var config docker.AuthConfiguration // default, tries docker hub w/o user/pass

	// if any configured host auths match task registry, try them (task docker auth can override)
	// TODO this is still a little hairy using suffix, we should probably try to parse it as a
	// url and extract the host (from both the config file & image)
	for _, v := range drv.auths {
		if reg != "" && strings.HasSuffix(v.ServerAddress, reg) {
			config = v
			break
		}
	}

	if task, ok := task.(Auther); ok {
		var err error
		_, span := trace.StartSpan(ctx, "docker_auth")
		config, err = task.DockerAuth()
		span.End()
		if err != nil {
			return err
		}
	}

	if reg != "" {
		config.ServerAddress = reg
	}

	// see if we already have it, if not, pull it
	_, err := drv.docker.InspectImage(ctx, task.Image())
	if err == docker.ErrNoSuchImage {
		err = drv.pullImage(ctx, task, config)
	}

	return err
}

func (drv *DockerDriver) pullImage(ctx context.Context, task drivers.ContainerTask, config docker.AuthConfiguration) error {
	log := common.Logger(ctx)
	reg, repo, tag := drivers.ParseImage(task.Image())
	globalRepo := path.Join(reg, repo)
	if reg != "" {
		config.ServerAddress = reg
	}

	var err error
	config.ServerAddress, err = registryURL(config.ServerAddress)
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{"registry": config.ServerAddress, "username": config.Username, "image": task.Image()}).Info("Pulling image")

	err = drv.docker.PullImage(docker.PullImageOptions{Repository: globalRepo, Tag: tag, Context: ctx}, config)
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

	attachSuccess := make(chan struct{})
	mwOut, mwErr := task.Logger()

	waiter, err := drv.docker.AttachToContainerNonBlocking(ctx, docker.AttachToContainerOptions{
		Success:      attachSuccess,
		Container:    container,
		OutputStream: mwOut,
		ErrorStream:  mwErr,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
		Stdin:        true,
		InputStream:  task.Input()})

	if err != nil && ctx.Err() == nil {
		// ignore if ctx has errored, rewrite status lay below
		return nil, err
	}

	// Sync up with NB Attacher above before starting the task
	if err == nil {
		// WARNING: the I/O below requires docker hijack function to honor
		// the contract below, specifically if an error is not returned
		// from AttachToContainerNonBlocking, then max blocking time
		// here should be what drv.docker dialer/client config was set to.
		<-attachSuccess
		attachSuccess <- struct{}{}
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
		Timestamp: strfmt.DateTime(ds.Read),
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

	// see if there's any healthcheck, and if so, wait for it to complete
	return drv.awaitHealthcheck(ctx, container)
}

func (drv *DockerDriver) awaitHealthcheck(ctx context.Context, container string) error {
	// inspect the container and check if there is any health check presented,
	// if there is, then wait for it to move to healthy before returning.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cont, err := drv.docker.InspectContainerWithContext(container, ctx)
		if err != nil {
			// TODO unknown fiddling to be had
			return err
		}

		// if no health check for this image (""), or it's healthy, then stop waiting.
		// state machine is "starting" -> "healthy" | "unhealthy"
		if cont.State.Health.Status == "" || cont.State.Health.Status == "healthy" {
			break
		}

		time.Sleep(100 * time.Millisecond) // avoid spin loop in case docker is actually fast
	}
	return nil
}

func (w *waitResult) wait(ctx context.Context) (status string, err error) {
	// wait retries internally until ctx is up, so we can ignore the error and
	// just say it was a timeout if we have [fatal] errors talking to docker, etc.
	// a more prevalent case is calling wait & container already finished, so again ignore err.
	exitCode, _ := w.drv.docker.WaitContainerWithContext(w.container, ctx)

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
		stats.Record(ctx, dockerOOMMeasure.M(1))
		common.Logger(ctx).Error("docker oom")
		err := errors.New("container out of memory, you may want to raise route.memory for this route (default: 128MB)")
		return drivers.StatusKilled, models.NewAPIError(http.StatusBadGateway, err)
	}
}
