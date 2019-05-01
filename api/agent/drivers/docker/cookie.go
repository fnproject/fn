package docker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

const (
	FnUserId  = 1000
	FnGroupId = 1000
)

var (
	ErrImageWithVolume = models.NewAPIError(http.StatusBadRequest, errors.New("image has Volume definition"))
	// FnDockerUser is used as the runtime user/group when running docker containers.
	// This is not configurable at the moment, because some fdks require that user/group to be present in the container.
	FnDockerUser = fmt.Sprintf("%v:%v", FnUserId, FnGroupId)
)

// A cookie identifies a unique request to run a task.
type cookie struct {
	// namespace id used from prefork pool if applicable
	poolId string
	// network name from docker networks if applicable
	netId string

	// docker container create options created by Driver.CreateCookie, required for Driver.Prepare()
	opts docker.CreateContainerOptions
	// task associated with this cookie
	task drivers.ContainerTask
	// pointer to docker driver
	drv *DockerDriver

	imgReg  string
	imgRepo string
	imgTag  string

	// contains inspected image if ValidateImage() is called
	image *CachedImage

	// contains created container if CreateContainer() is called
	container *docker.Container
}

func (c *cookie) configureImage(log logrus.FieldLogger) {
	c.imgReg, c.imgRepo, c.imgTag = drivers.ParseImage(c.task.Image())
}

func (c *cookie) configureLabels(log logrus.FieldLogger) {
	if c.drv.conf.ContainerLabelTag == "" {
		return
	}

	if c.opts.Config.Labels == nil {
		c.opts.Config.Labels = make(map[string]string)
	}

	c.opts.Config.Labels[FnAgentClassifierLabel] = c.drv.conf.ContainerLabelTag
	c.opts.Config.Labels[FnAgentInstanceLabel] = c.drv.instanceId
}

func (c *cookie) configureLogger(log logrus.FieldLogger) {

	conf := c.task.LoggerConfig()
	if conf.URL == "" {
		c.opts.HostConfig.LogConfig = docker.LogConfig{
			Type: "none",
		}
		return
	}

	c.opts.HostConfig.LogConfig = docker.LogConfig{
		Type: "syslog",
		Config: map[string]string{
			"syslog-address":  conf.URL,
			"syslog-facility": "user",
			"syslog-format":   "rfc5424",
		},
	}

	tags := make([]string, 0, len(conf.Tags))
	for _, pair := range conf.Tags {
		tags = append(tags, fmt.Sprintf("%s=%s", pair.Name, pair.Value))
	}
	if len(tags) > 0 {
		c.opts.HostConfig.LogConfig.Config["tag"] = strings.Join(tags, ",")
	}
}

func (c *cookie) configureMem(log logrus.FieldLogger) {
	if c.task.Memory() == 0 {
		return
	}

	mem := int64(c.task.Memory())

	c.opts.Config.Memory = mem
	c.opts.Config.MemorySwap = mem // disables swap
	c.opts.Config.KernelMemory = mem
	c.opts.HostConfig.MemorySwap = mem
	c.opts.HostConfig.KernelMemory = mem
	var zero int64
	c.opts.HostConfig.MemorySwappiness = &zero // disables host swap
}

func (c *cookie) configureFsSize(log logrus.FieldLogger) {
	if c.task.FsSize() == 0 {
		return
	}

	// If defined, impose file system size limit. In MB units.
	if c.opts.HostConfig.StorageOpt == nil {
		c.opts.HostConfig.StorageOpt = make(map[string]string)
	}

	opt := fmt.Sprintf("%vM", c.task.FsSize())
	log.WithFields(logrus.Fields{"size": opt, "call_id": c.task.Id()}).Debug("setting storage option")
	c.opts.HostConfig.StorageOpt["size"] = opt
}

func (c *cookie) configurePIDs(log logrus.FieldLogger) {
	pids := c.task.PIDs()
	if pids == 0 {
		return
	}

	pids64 := int64(pids)
	log.WithFields(logrus.Fields{"pids": pids64, "call_id": c.task.Id()}).Debug("setting PIDs")
	c.opts.HostConfig.PidsLimit = &pids64
}

// configureOpenFiles will set the ULimit for `nofile` on the Docker container
func (c *cookie) configureOpenFiles(log logrus.FieldLogger) {
	openFiles := c.task.OpenFiles()
	if openFiles == 0 {
		return
	}

	openFiles64 := int64(openFiles)
	log.WithFields(logrus.Fields{"openFiles": openFiles64, "call_id": c.task.Id()}).Debug("setting open files")
	c.addULimit(docker.ULimit{Name: "nofile", Soft: openFiles64, Hard: openFiles64})
}

// configureLockedMemory will set the ULimit for `memlock` on the Docker container
func (c *cookie) configureLockedMemory(log logrus.FieldLogger) {
	lockedMemory := c.task.LockedMemory()
	if lockedMemory == 0 {
		return
	}

	lockedMemory64 := int64(lockedMemory)
	log.WithFields(logrus.Fields{"lockedMemory": lockedMemory64, "call_id": c.task.Id()}).Debug("setting locked memory")
	c.addULimit(docker.ULimit{Name: "memlock", Soft: lockedMemory64, Hard: lockedMemory64})
}

// configurePendingSignals will set the ULimit for `sigpending` on the Docker
// container
func (c *cookie) configurePendingSignals(log logrus.FieldLogger) {
	pendingSignals := c.task.PendingSignals()
	if pendingSignals == 0 {
		return
	}

	pendingSignals64 := int64(pendingSignals)
	log.WithFields(logrus.Fields{"pendingSignals": pendingSignals64, "call_id": c.task.Id()}).Debug("setting pending signals")
	c.addULimit(docker.ULimit{Name: "sigpending", Soft: pendingSignals64, Hard: pendingSignals64})
}

// configureMessageQueue will set the ULimit for `msqueue` on the Docker
// container
func (c *cookie) configureMessageQueue(log logrus.FieldLogger) {
	messageQueue := c.task.MessageQueue()
	if messageQueue == 0 {
		return
	}

	messageQueue64 := int64(messageQueue)
	log.WithFields(logrus.Fields{"messageQueue": messageQueue64, "call_id": c.task.Id()}).Debug("setting message queue")
	c.addULimit(docker.ULimit{Name: "msqueue", Soft: messageQueue64, Hard: messageQueue64})
}

func (c *cookie) configureTmpFs(log logrus.FieldLogger) {
	// if RO Root is NOT enabled and TmpFsSize does not have any limit, then we do not need
	// any tmpfs in the container since function can freely write whereever it wants.
	if c.task.TmpFsSize() == 0 && !c.drv.conf.EnableReadOnlyRootFs {
		return
	}

	if c.opts.HostConfig.Tmpfs == nil {
		c.opts.HostConfig.Tmpfs = make(map[string]string)
	}

	var tmpFsOption string
	if c.task.TmpFsSize() != 0 {
		if c.drv.conf.MaxTmpFsInodes != 0 {
			tmpFsOption = fmt.Sprintf("size=%dm,nr_inodes=%d", c.task.TmpFsSize(), c.drv.conf.MaxTmpFsInodes)
		} else {
			tmpFsOption = fmt.Sprintf("size=%dm", c.task.TmpFsSize())
		}
	}

	log.WithFields(logrus.Fields{"target": "/tmp", "options": tmpFsOption, "call_id": c.task.Id()}).Debug("setting tmpfs")
	c.opts.HostConfig.Tmpfs["/tmp"] = tmpFsOption
}

func (c *cookie) configureIOFS(log logrus.FieldLogger) {
	path := c.task.UDSDockerPath()
	if path == "" {
		// TODO this should be required soon-ish
		return
	}

	bind := fmt.Sprintf("%s:%s", path, c.task.UDSDockerDest())
	c.opts.HostConfig.Binds = append(c.opts.HostConfig.Binds, bind)
	log.WithFields(logrus.Fields{"bind": bind, "call_id": c.task.Id()}).Debug("setting bind")
}

func (c *cookie) configureVolumes(log logrus.FieldLogger) {
	if len(c.task.Volumes()) == 0 {
		return
	}

	if c.opts.Config.Volumes == nil {
		c.opts.Config.Volumes = map[string]struct{}{}
	}

	for _, mapping := range c.task.Volumes() {
		hostDir := mapping[0]
		containerDir := mapping[1]
		c.opts.Config.Volumes[containerDir] = struct{}{}
		mapn := fmt.Sprintf("%s:%s", hostDir, containerDir)
		c.opts.HostConfig.Binds = append(c.opts.HostConfig.Binds, mapn)
		log.WithFields(logrus.Fields{"volumes": mapn, "call_id": c.task.Id()}).Debug("setting volumes")
	}
}

func (c *cookie) configureCPU(log logrus.FieldLogger) {
	// Translate milli cpus into CPUQuota & CPUPeriod (see Linux cGroups CFS cgroup v1 documentation)
	// eg: task.CPUQuota() of 8000 means CPUQuota of 8 * 100000 usecs in 100000 usec period,
	// which is approx 8 CPUS in CFS world.
	// Also see docker run options --cpu-quota and --cpu-period
	if c.task.CPUs() == 0 {
		return
	}

	quota := int64(c.task.CPUs() * 100)
	period := int64(100000)

	log.WithFields(logrus.Fields{"quota": quota, "period": period, "call_id": c.task.Id()}).Debug("setting CPU")
	c.opts.HostConfig.CPUQuota = quota
	c.opts.HostConfig.CPUPeriod = period
}

func (c *cookie) configureWorkDir(log logrus.FieldLogger) {
	wd := c.task.WorkDir()
	if wd == "" {
		return
	}

	log.WithFields(logrus.Fields{"wd": wd, "call_id": c.task.Id()}).Debug("setting work dir")
	c.opts.Config.WorkingDir = wd
}

func (c *cookie) configureNetwork(log logrus.FieldLogger) {
	if c.opts.HostConfig.NetworkMode != "" {
		return
	}

	if c.task.DisableNet() {
		c.opts.HostConfig.NetworkMode = "none"
		return
	}

	// If pool is enabled, we try to pick network from pool
	if c.drv.pool != nil {
		id, err := c.drv.pool.AllocPoolId()
		if id != "" {
			// We are able to fetch a container from pool. Now, use its
			// network, ipc and pid namespaces.
			c.opts.HostConfig.NetworkMode = fmt.Sprintf("container:%s", id)
			//c.opts.HostConfig.IpcMode = linker
			//c.opts.HostConfig.PidMode = linker
			c.poolId = id
			return
		}
		if err != nil {
			log.WithError(err).Error("Could not fetch pre fork pool container")
		}
	}

	// if pool is not enabled or fails, then pick from defined networks if any
	id := c.drv.network.AllocNetwork()
	if id != "" {
		c.opts.HostConfig.NetworkMode = id
		c.netId = id
	}
}

func (c *cookie) configureHostname(log logrus.FieldLogger) {
	// hostname and container NetworkMode is not compatible.
	if c.opts.HostConfig.NetworkMode != "" {
		return
	}

	log.WithFields(logrus.Fields{"hostname": c.drv.hostname, "call_id": c.task.Id()}).Debug("setting hostname")
	c.opts.Config.Hostname = c.drv.hostname
}

func (c *cookie) configureCmd(log logrus.FieldLogger) {
	if c.task.Command() == "" {
		return
	}

	// NOTE: this is hyper-sensitive and may not be correct like this even, but it passes old tests
	cmd := strings.Fields(c.task.Command())
	log.WithFields(logrus.Fields{"call_id": c.task.Id(), "cmd": cmd, "len": len(cmd)}).Debug("docker command")
	c.opts.Config.Cmd = cmd
}

func (c *cookie) configureEnv(log logrus.FieldLogger) {
	if len(c.task.EnvVars()) == 0 {
		return
	}

	if c.opts.Config.Env == nil {
		c.opts.Config.Env = make([]string, 0, len(c.task.EnvVars()))
	}

	for name, val := range c.task.EnvVars() {
		c.opts.Config.Env = append(c.opts.Config.Env, name+"="+val)
	}
}

func (c *cookie) configureSecurity(log logrus.FieldLogger) {
	if c.drv.conf.DisableUnprivilegedContainers {
		return
	}
	c.opts.Config.User = FnDockerUser
	c.opts.HostConfig.CapDrop = []string{"all"}
	c.opts.HostConfig.SecurityOpt = []string{"no-new-privileges:true"}
	log.WithFields(logrus.Fields{"user": c.opts.Config.User,
		"CapDrop": c.opts.HostConfig.CapDrop, "SecurityOpt": c.opts.HostConfig.SecurityOpt, "call_id": c.task.Id()}).Debug("setting security")
}

// addULimit adds lim to the docker host config ulimits slice, optionally
// creating the slice if it isn't created yet.
func (c *cookie) addULimit(lim docker.ULimit) {
	limits := c.opts.HostConfig.Ulimits
	if limits == nil {
		limits = []docker.ULimit{}
	}

	c.opts.HostConfig.Ulimits = append(limits, lim)
}

// implements Cookie
func (c *cookie) Close(ctx context.Context) error {
	var err error
	if c.container != nil {
		err = c.drv.docker.RemoveContainer(docker.RemoveContainerOptions{
			ID: c.task.Id(), Force: true, RemoveVolumes: true, Context: ctx})
		if err != nil {
			common.Logger(ctx).WithError(err).WithFields(logrus.Fields{"call_id": c.task.Id()}).Error("error removing container")
		}
	}

	if c.poolId != "" && c.drv.pool != nil {
		c.drv.pool.FreePoolId(c.poolId)
	}
	if c.netId != "" {
		c.drv.network.FreeNetwork(c.netId)
	}

	if c.image != nil && c.drv.imgCache != nil {
		c.drv.imgCache.MarkFree(c.image)
	}
	return err
}

// implements Cookie
func (c *cookie) Run(ctx context.Context) (drivers.WaitResult, error) {
	return c.drv.run(ctx, c.task.Id(), c.task)
}

// implements Cookie
func (c *cookie) ContainerOptions() interface{} {
	return c.opts
}

// implements Cookie
func (c *cookie) Freeze(ctx context.Context) error {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "Freeze"})
	log.WithFields(logrus.Fields{"call_id": c.task.Id()}).Debug("docker pause")

	err := c.drv.docker.PauseContainer(c.task.Id(), ctx)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"call_id": c.task.Id()}).Error("error pausing container")
	}
	return err
}

// implements Cookie
func (c *cookie) Unfreeze(ctx context.Context) error {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "Unfreeze"})
	log.WithFields(logrus.Fields{"call_id": c.task.Id()}).Debug("docker unpause")

	err := c.drv.docker.UnpauseContainer(c.task.Id(), ctx)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"call_id": c.task.Id()}).Error("error unpausing container")
	}
	return err
}

func (c *cookie) authImage(ctx context.Context) (*docker.AuthConfiguration, error) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "AuthImage"})
	log.WithFields(logrus.Fields{"call_id": c.task.Id()}).Debug("docker auth image")

	// ask for docker creds before looking for image, as the tasker may need to
	// validate creds even if the image is downloaded.
	config := findRegistryConfig(c.imgReg, c.drv.auths)

	if task, ok := c.task.(Auther); ok {
		_, span := trace.StartSpan(ctx, "docker_auth")
		authConfig, err := task.DockerAuth(ctx, c.task.Image())
		span.End()
		if err != nil {
			return nil, err
		}
		if authConfig != nil {
			config = authConfig
		}
	}

	return config, nil
}

// implements Cookie
func (c *cookie) ValidateImage(ctx context.Context) (bool, error) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "ValidateImage"})
	log.WithFields(logrus.Fields{"call_id": c.task.Id(), "image": c.task.Image()}).Debug("docker inspect image")

	if c.image != nil {
		return false, nil
	}

	// see if we already have it
	// TODO this should use the image cache instead of making a docker call
	img, err := c.drv.docker.InspectImage(ctx, c.task.Image())
	if err == docker.ErrNoSuchImage {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// check image doesn't have Volumes
	if !c.drv.conf.ImageEnableVolume && img.Config != nil && len(img.Config.Volumes) > 0 {
		err = ErrImageWithVolume
	}

	c.image = &CachedImage{
		ID:       img.ID,
		ParentID: img.Parent,
		RepoTags: img.RepoTags,
		Size:     uint64(img.Size),
	}

	if c.drv.imgCache != nil {
		if err == ErrImageWithVolume {
			c.drv.imgCache.Update(c.image)
		} else {
			c.drv.imgCache.MarkBusy(c.image)
		}
	}
	return false, err
}

// implements Cookie
func (c *cookie) PullImage(ctx context.Context) error {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "PullImage"})
	if c.image != nil {
		return nil
	}

	cfg, err := c.authImage(ctx)
	if err != nil {
		return err
	}

	repo := path.Join(c.imgReg, c.imgRepo)

	log = common.Logger(ctx).WithFields(logrus.Fields{"registry": cfg.ServerAddress, "username": cfg.Username})
	log.WithFields(logrus.Fields{"call_id": c.task.Id(), "image": c.task.Image()}).Debug("docker pull")
	ctx = common.WithLogger(ctx, log)

	errC := c.drv.imgPuller.PullImage(ctx, cfg, c.task.Image(), repo, c.imgTag)
	return <-errC
}

// implements Cookie
func (c *cookie) CreateContainer(ctx context.Context) error {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "CreateContainer"})
	log.WithFields(logrus.Fields{"call_id": c.task.Id(), "image": c.task.Image()}).Debug("docker create container")

	if c.image == nil {
		log.Fatal("invalid usage: image not validated")
	}
	if c.container != nil {
		return nil
	}

	var err error

	createOptions := c.opts
	createOptions.Context = ctx

	c.container, err = c.drv.docker.CreateContainer(createOptions)

	// IMPORTANT: The return code 503 here is controversial. Here we treat disk pressure as a temporary
	// service too busy event that will likely to correct itself. Here with 503 we allow this request
	// to land on another (or back to same runner) which will likely to succeed. We have received
	// docker.ErrNoSuchImage because just after PullImage(), image cleaner (or manual intervention)
	// must have removed this image.
	if err == docker.ErrNoSuchImage {
		log.WithError(err).Error("Cannot CreateContainer image likely removed")
		return models.ErrCallTimeoutServerBusy
	}

	if err != nil {
		log.WithError(err).Error("Could not create container")
		return err
	}

	return nil
}

var _ drivers.Cookie = &cookie{}
