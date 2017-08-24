package containerd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/naming"
	"github.com/docker/swarmkit/log"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var (
	devNull                    *os.File
	errAdapterNotPrepared      = errors.New("container adapter not prepared")
	mountPropagationReverseMap = map[api.Mount_BindOptions_MountPropagation]string{
		api.MountPropagationPrivate:  "private",
		api.MountPropagationRPrivate: "rprivate",
		api.MountPropagationShared:   "shared",
		api.MountPropagationRShared:  "rshared",
		api.MountPropagationRSlave:   "slave",
		api.MountPropagationSlave:    "rslave",
	}
)

// containerAdapter conducts remote operations for a container. All calls
// are mostly naked calls to the client API, seeded with information from
// containerConfig.
type containerAdapter struct {
	client     *containerd.Client
	spec       *api.ContainerSpec
	secrets    exec.SecretGetter
	name       string
	image      containerd.Image // Pulled image
	container  containerd.Container
	task       containerd.Task
	exitStatus error
}

func newContainerAdapter(client *containerd.Client, task *api.Task, secrets exec.SecretGetter) (*containerAdapter, error) {
	spec := task.Spec.GetContainer()
	if spec == nil {
		return nil, exec.ErrRuntimeUnsupported
	}

	c := &containerAdapter{
		client:  client,
		spec:    spec,
		secrets: secrets,
		name:    naming.Task(task),
	}

	if err := c.reattach(context.Background()); err != nil {
		return nil, err
	}

	return c, nil
}

// reattaches to an existing container. If the container is found but
// the task is missing then still succeeds, allowing subsequent use of
// c.delete()
func (c *containerAdapter) reattach(ctx context.Context) error {
	container, err := c.client.LoadContainer(ctx, c.name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			c.log(ctx).Debug("reattach: container not found")
			return nil
		}

		return errors.Wrap(err, "reattach: loading container")
	}
	c.log(ctx).Debug("reattach: loaded container")
	c.container = container

	// TODO(ijc) Consider an addition to container library which
	// directly attaches stdin to /dev/null.
	if devNull == nil {
		if devNull, err = os.Open(os.DevNull); err != nil {
			return errors.Wrap(err, "reattach: opening null device")
		}
	}

	task, err := container.Task(ctx, containerd.WithAttach(devNull, os.Stdout, os.Stderr))
	if err != nil {
		if errdefs.IsNotFound(err) {
			c.log(ctx).WithError(err).Info("reattach: no running task")
			return nil
		}
		return errors.Wrap(err, "reattach: reattaching task")
	}
	c.task = task
	c.log(ctx).Debug("reattach: successful")
	return nil
}

func (c *containerAdapter) log(ctx context.Context) *logrus.Entry {
	return log.G(ctx).WithFields(logrus.Fields{
		"container.id": c.name,
	})
}

func (c *containerAdapter) pullImage(ctx context.Context) error {
	image, err := c.client.Pull(ctx, c.spec.Image, containerd.WithPullUnpack)
	if err != nil {
		return errors.Wrap(err, "pulling container image")
	}
	c.image = image

	return nil
}

func withMounts(ctx context.Context, ms []api.Mount) containerd.SpecOpts {
	sort.Sort(mounts(ms))

	return func(s *specs.Spec) error {
		for _, m := range ms {
			if !filepath.IsAbs(m.Target) {
				return errors.Errorf("mount %s is not absolute", m.Target)
			}

			switch m.Type {
			case api.MountTypeTmpfs:
				opts := []string{"noexec", "nosuid", "nodev", "rprivate"}
				if m.TmpfsOptions != nil {
					if m.TmpfsOptions.SizeBytes <= 0 {
						return errors.New("invalid tmpfs size give")
					}
					opts = append(opts, fmt.Sprintf("size=%d", m.TmpfsOptions.SizeBytes))
					opts = append(opts, fmt.Sprintf("mode=%o", m.TmpfsOptions.Mode))
				}
				if m.ReadOnly {
					opts = append(opts, "ro")
				} else {
					opts = append(opts, "rw")
				}

				s.Mounts = append(s.Mounts, specs.Mount{
					Destination: m.Target,
					Type:        "tmpfs",
					Source:      "tmpfs",
					Options:     opts,
				})

			case api.MountTypeVolume:
				return errors.Errorf("volume mounts not implemented, ignoring %v", m)

			case api.MountTypeBind:
				opts := []string{"rbind"}
				if m.ReadOnly {
					opts = append(opts, "ro")
				} else {
					opts = append(opts, "rw")
				}

				propagation := "rprivate"
				if m.BindOptions != nil {
					if p, ok := mountPropagationReverseMap[m.BindOptions.Propagation]; ok {
						propagation = p
					} else {
						log.G(ctx).Warningf("unknown bind mount propagation, using %q", propagation)
					}
				}
				opts = append(opts, propagation)

				s.Mounts = append(s.Mounts, specs.Mount{
					Destination: m.Target,
					Type:        "bind",
					Source:      m.Source,
					Options:     opts,
				})
			}
		}
		return nil
	}
}

func (c *containerAdapter) isPrepared() bool {
	return c.container != nil && c.task != nil
}

func (c *containerAdapter) prepare(ctx context.Context) error {
	if c.isPrepared() {
		return errors.New("adapter already prepared")
	}
	if c.image == nil {
		return errors.New("image has not been pulled")
	}

	specOpts := []containerd.SpecOpts{
		containerd.WithImageConfig(ctx, c.image),
		withMounts(ctx, c.spec.Mounts),
	}

	// spec.Process.Args is config.Entrypoint + config.Cmd at this
	// point from WithImageConfig above. If the ContainerSpec
	// specifies a Command then we can completely override. If it
	// does not then all we can do is append our Args and hope
	// they do not conflict.
	// TODO(ijc) Improve this
	if len(c.spec.Command) > 0 {
		args := append(c.spec.Command, c.spec.Args...)
		specOpts = append(specOpts, containerd.WithProcessArgs(args...))
	} else {
		specOpts = append(specOpts, func(s *specs.Spec) error {
			s.Process.Args = append(s.Process.Args, c.spec.Args...)
			return nil
		})
	}

	spec, err := containerd.GenerateSpec(specOpts...)
	if err != nil {
		return err
	}

	// TODO(ijc) Consider an addition to container library which
	// directly attaches stdin to /dev/null.
	if devNull == nil {
		if devNull, err = os.Open(os.DevNull); err != nil {
			return errors.Wrap(err, "opening null device")
		}
	}

	c.container, err = c.client.NewContainer(ctx, c.name,
		containerd.WithSpec(spec),
		containerd.WithNewSnapshot(c.name, c.image))
	if err != nil {
		return errors.Wrap(err, "creating container")
	}

	// TODO(ijc) support ControllerLogs interface.
	io := containerd.NewIOWithTerminal(devNull, os.Stdout, os.Stderr, spec.Process.Terminal)

	c.task, err = c.container.NewTask(ctx, io)
	if err != nil {
		// Destroy the container we created above, but
		// propagate the original error.
		if err2 := c.container.Delete(ctx); err2 != nil {
			c.log(ctx).WithError(err2).Error("failed to delete container on prepare failure")
		}
		c.container = nil
		return errors.Wrap(err, "creating task")
	}

	return nil
}

func (c *containerAdapter) start(ctx context.Context) error {
	if !c.isPrepared() {
		return errAdapterNotPrepared
	}
	err := c.task.Start(ctx)
	return errors.Wrap(err, "starting")
}

func (c *containerAdapter) wait(ctx context.Context) error {
	if !c.isPrepared() {
		return errAdapterNotPrepared
	}
	status, err := c.task.Wait(ctx)
	if err != nil {
		return errors.Wrap(err, "waiting")
	}
	// Should update c.exitStatus or not?
	return makeExitError(status, "")
}

type status struct {
	ID         string
	Pid        uint32
	Status     containerd.Status
	ExitStatus error
}

func (c *containerAdapter) inspect(ctx context.Context) (status, error) {
	if !c.isPrepared() {
		return status{}, errAdapterNotPrepared
	}

	ts, err := c.task.Status(ctx)
	if err != nil {
		return status{}, err
	}
	s := status{
		ID:         c.container.ID(),
		Pid:        c.task.Pid(),
		Status:     ts,
		ExitStatus: c.exitStatus,
	}
	return s, nil
}

func (c *containerAdapter) shutdown(ctx context.Context) error {
	if !c.isPrepared() {
		return errAdapterNotPrepared
	}

	var (
		sig     syscall.Signal
		timeout = time.Duration(10 * time.Second)
		err     error
	)

	if c.spec.StopSignal != "" {
		if sig, err = signal.ParseSignal(c.spec.StopSignal); err != nil {
			sig = syscall.SIGTERM
			c.log(ctx).WithError(err).Errorf("unknown StopSignal, using %q", sig)
		}
	} else {
		sig = syscall.SIGTERM
		c.log(ctx).Infof("no StopSignal given, using %q", sig)
	}

	if c.spec.StopGracePeriod != nil {
		timeout, _ = gogotypes.DurationFromProto(c.spec.StopGracePeriod)
	}

	deleteErr := make(chan error, 1)
	deleteCtx, deleteCancel := context.WithCancel(ctx)
	defer deleteCancel()

	go func(ctx context.Context, ch chan error) {
		status, err := c.task.Delete(ctx)
		if err != nil {
			c.log(ctx).WithError(err).Debug("Task.Delete failed")
			ch <- err
		}
		c.log(ctx).Debugf("Task.Delete success, status=%d", status)
		ch <- makeExitError(status, "")
	}(deleteCtx, deleteErr)

	c.log(ctx).Debugf("Killing task with %q signal", sig)
	if err := c.task.Kill(ctx, sig); err != nil {
		return errors.Wrapf(err, "killing task with %q", sig)
	}

	select {
	case c.exitStatus = <-deleteErr:
		return c.exitStatus
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		c.log(ctx).Infof("Task did not exit after %s", timeout)
		// Fall through
	}

	if sig == syscall.SIGKILL {
		// We've tried as hard as we can.
		return errors.New("task is unkillable")
	}

	// Bring out the big guns
	sig = syscall.SIGKILL
	c.log(ctx).Debugf("Killing task harder with %q signal", sig)
	if err := c.task.Kill(ctx, sig); err != nil {
		return errors.Wrapf(err, "killing task with %q", sig)
	}

	select {
	case c.exitStatus = <-deleteErr:
		return c.exitStatus
	case <-ctx.Done():
		return ctx.Err()
	}

}

func (c *containerAdapter) terminate(ctx context.Context) error {
	if !c.isPrepared() {
		return errAdapterNotPrepared
	}

	c.log(ctx).Debug("Terminate")
	return errors.New("terminate not implemented")
}

func (c *containerAdapter) remove(ctx context.Context) error {
	// Unlike most other entry points we don't use c.isPrepared
	// here so that we can clean up a container which was
	// partially reattached (via c.attach).
	if c.container == nil {
		return errAdapterNotPrepared
	}

	c.log(ctx).Debug("Remove")
	err := c.container.Delete(ctx)
	return errors.Wrap(err, "removing container")
}

func isContainerCreateNameConflict(err error) bool {
	// container ".*" already exists
	splits := strings.SplitN(err.Error(), "\"", 3)
	return splits[0] == "container " && splits[2] == " already exists"
}

func isUnknownContainer(err error) bool {
	return strings.Contains(err.Error(), "container does not exist")
}

// For sort.Sort
type mounts []api.Mount

// Len returns the number of mounts. Used in sorting.
func (m mounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m mounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m mounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m mounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Target), string(os.PathSeparator))
}
