package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
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

// implements Cookie
func (c *cookie) Close(ctx context.Context) error {
	err := c.drv.removeContainer(ctx, c.task.Id())
	c.drv.unpickPool(c)
	c.drv.unpickNetwork(c)
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
		logrus.WithError(err).WithFields(logrus.Fields{"call_id": c.task.Id()}).Error("error pausing container")
	}
	return err
}

// implements Cookie
func (c *cookie) Unfreeze(ctx context.Context) error {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "Unfreeze"})
	log.WithFields(logrus.Fields{"call_id": c.task.Id()}).Debug("docker unpause")

	err := c.drv.docker.UnpauseContainer(c.task.Id(), ctx)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"call_id": c.task.Id()}).Error("error unpausing container")
	}
	return err
}

var _ drivers.Cookie = &cookie{}
