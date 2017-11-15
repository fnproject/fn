package containerd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	goruntime "runtime"
	"strings"
	"sync"
	"syscall"

	eventsapi "github.com/containerd/containerd/api/services/events/v1"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/typeurl"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// UnknownExitStatus is returned when containerd is unable to
// determine the exit status of a process. This can happen if the process never starts
// or if an error was encountered when obtaining the exit status, it is set to 255.
const UnknownExitStatus = 255

// Status returns process status and exit information
type Status struct {
	// Status of the process
	Status ProcessStatus
	// ExitStatus returned by the process
	ExitStatus uint32
}

type ProcessStatus string

const (
	// Running indicates the process is currently executing
	Running ProcessStatus = "running"
	// Created indicates the process has been created within containerd but the
	// user's defined process has not started
	Created ProcessStatus = "created"
	// Stopped indicates that the process has ran and exited
	Stopped ProcessStatus = "stopped"
	// Paused indicates that the process is currently paused
	Paused ProcessStatus = "paused"
	// Pausing indicates that the process is currently switching from a
	// running state into a paused state
	Pausing ProcessStatus = "pausing"
	// Unknown indicates that we could not determine the status from the runtime
	Unknown ProcessStatus = "unknown"
)

// IOCloseInfo allows specific io pipes to be closed on a process
type IOCloseInfo struct {
	Stdin bool
}

// IOCloserOpts allows the caller to set specific pipes as closed on a process
type IOCloserOpts func(*IOCloseInfo)

// WithStdinCloser closes the stdin of a process
func WithStdinCloser(r *IOCloseInfo) {
	r.Stdin = true
}

// CheckpointTaskInfo allows specific checkpoint information to be set for the task
type CheckpointTaskInfo struct {
	// ParentCheckpoint is the digest of a parent checkpoint
	ParentCheckpoint digest.Digest
	// Options hold runtime specific settings for checkpointing a task
	Options interface{}
}

// CheckpointTaskOpts allows the caller to set checkpoint options
type CheckpointTaskOpts func(*CheckpointTaskInfo) error

// TaskInfo sets options for task creation
type TaskInfo struct {
	// Checkpoint is the Descriptor for an existing checkpoint that can be used
	// to restore a task's runtime and memory state
	Checkpoint *types.Descriptor
	// RootFS is a list of mounts to use as the task's root filesystem
	RootFS []mount.Mount
	// Options hold runtime specific settings for task creation
	Options interface{}
}

// Task is the executable object within containerd
type Task interface {
	Process

	// Pause suspends the execution of the task
	Pause(context.Context) error
	// Resume the execution of the task
	Resume(context.Context) error
	// Exec creates a new process inside the task
	Exec(context.Context, string, *specs.Process, IOCreation) (Process, error)
	// Pids returns a list of system specific process ids inside the task
	Pids(context.Context) ([]uint32, error)
	// Checkpoint serializes the runtime and memory information of a task into an
	// OCI Index that can be push and pulled from a remote resource.
	//
	// Additional software like CRIU maybe required to checkpoint and restore tasks
	Checkpoint(context.Context, ...CheckpointTaskOpts) (v1.Descriptor, error)
	// Update modifies executing tasks with updated settings
	Update(context.Context, ...UpdateTaskOpts) error
}

var _ = (Task)(&task{})

type task struct {
	client *Client

	io  *IO
	id  string
	pid uint32

	mu       sync.Mutex
	deferred *tasks.CreateTaskRequest
}

// Pid returns the pid or process id for the task
func (t *task) Pid() uint32 {
	return t.pid
}

func (t *task) Start(ctx context.Context) error {
	t.mu.Lock()
	deferred := t.deferred
	t.mu.Unlock()
	if deferred != nil {
		response, err := t.client.TaskService().Create(ctx, deferred)
		t.mu.Lock()
		t.deferred = nil
		t.mu.Unlock()
		if err != nil {
			t.io.closer.Close()
			return err
		}
		t.pid = response.Pid
		return nil
	}
	_, err := t.client.TaskService().Start(ctx, &tasks.StartRequest{
		ContainerID: t.id,
	})
	if err != nil {
		t.io.closer.Close()
	}
	return err
}

func (t *task) Kill(ctx context.Context, s syscall.Signal) error {
	_, err := t.client.TaskService().Kill(ctx, &tasks.KillRequest{
		Signal:      uint32(s),
		ContainerID: t.id,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (t *task) Pause(ctx context.Context) error {
	_, err := t.client.TaskService().Pause(ctx, &tasks.PauseTaskRequest{
		ContainerID: t.id,
	})
	return errdefs.FromGRPC(err)
}

func (t *task) Resume(ctx context.Context) error {
	_, err := t.client.TaskService().Resume(ctx, &tasks.ResumeTaskRequest{
		ContainerID: t.id,
	})
	return errdefs.FromGRPC(err)
}

func (t *task) Status(ctx context.Context) (Status, error) {
	r, err := t.client.TaskService().Get(ctx, &tasks.GetRequest{
		ContainerID: t.id,
	})
	if err != nil {
		return Status{}, errdefs.FromGRPC(err)
	}
	return Status{
		Status:     ProcessStatus(strings.ToLower(r.Process.Status.String())),
		ExitStatus: r.Process.ExitStatus,
	}, nil
}

func (t *task) Wait(ctx context.Context) (uint32, error) {
	cancellable, cancel := context.WithCancel(ctx)
	defer cancel()
	eventstream, err := t.client.EventService().Subscribe(cancellable, &eventsapi.SubscribeRequest{
		Filters: []string{"topic==" + runtime.TaskExitEventTopic},
	})
	if err != nil {
		return UnknownExitStatus, errdefs.FromGRPC(err)
	}
	t.mu.Lock()
	checkpoint := t.deferred != nil
	t.mu.Unlock()
	if !checkpoint {
		// first check if the task has exited
		status, err := t.Status(ctx)
		if err != nil {
			return UnknownExitStatus, errdefs.FromGRPC(err)
		}
		if status.Status == Stopped {
			return status.ExitStatus, nil
		}
	}
	for {
		evt, err := eventstream.Recv()
		if err != nil {
			return UnknownExitStatus, err
		}
		if typeurl.Is(evt.Event, &eventsapi.TaskExit{}) {
			v, err := typeurl.UnmarshalAny(evt.Event)
			if err != nil {
				return UnknownExitStatus, err
			}
			e := v.(*eventsapi.TaskExit)
			if e.ContainerID == t.id && e.Pid == t.pid {
				return e.ExitStatus, nil
			}
		}
	}
}

// Delete deletes the task and its runtime state
// it returns the exit status of the task and any errors that were encountered
// during cleanup
func (t *task) Delete(ctx context.Context, opts ...ProcessDeleteOpts) (uint32, error) {
	for _, o := range opts {
		if err := o(ctx, t); err != nil {
			return UnknownExitStatus, err
		}
	}
	status, err := t.Status(ctx)
	if err != nil && errdefs.IsNotFound(err) {
		return UnknownExitStatus, err
	}
	switch status.Status {
	case Stopped, Unknown, "":
	default:
		return UnknownExitStatus, errors.Wrapf(errdefs.ErrFailedPrecondition, "task must be stopped before deletion: %s", status.Status)
	}
	if t.io != nil {
		t.io.Cancel()
		t.io.Wait()
		t.io.Close()
	}
	r, err := t.client.TaskService().Delete(ctx, &tasks.DeleteTaskRequest{
		ContainerID: t.id,
	})
	if err != nil {
		return UnknownExitStatus, err
	}
	return r.ExitStatus, nil
}

func (t *task) Exec(ctx context.Context, id string, spec *specs.Process, ioCreate IOCreation) (Process, error) {
	if id == "" {
		return nil, errors.Wrapf(errdefs.ErrInvalidArgument, "exec id must not be empty")
	}
	i, err := ioCreate(id)
	if err != nil {
		return nil, err
	}
	any, err := typeurl.MarshalAny(spec)
	if err != nil {
		return nil, err
	}
	request := &tasks.ExecProcessRequest{
		ContainerID: t.id,
		ExecID:      id,
		Terminal:    i.Terminal,
		Stdin:       i.Stdin,
		Stdout:      i.Stdout,
		Stderr:      i.Stderr,
		Spec:        any,
	}
	if _, err := t.client.TaskService().Exec(ctx, request); err != nil {
		i.Cancel()
		i.Wait()
		i.Close()
		return nil, err
	}
	return &process{
		id:   id,
		task: t,
		io:   i,
		spec: spec,
	}, nil
}

func (t *task) Pids(ctx context.Context) ([]uint32, error) {
	response, err := t.client.TaskService().ListPids(ctx, &tasks.ListPidsRequest{
		ContainerID: t.id,
	})
	if err != nil {
		return nil, err
	}
	return response.Pids, nil
}

func (t *task) CloseIO(ctx context.Context, opts ...IOCloserOpts) error {
	r := &tasks.CloseIORequest{
		ContainerID: t.id,
	}
	var i IOCloseInfo
	for _, o := range opts {
		o(&i)
	}
	r.Stdin = i.Stdin
	_, err := t.client.TaskService().CloseIO(ctx, r)
	return err
}

func (t *task) IO() *IO {
	return t.io
}

func (t *task) Resize(ctx context.Context, w, h uint32) error {
	_, err := t.client.TaskService().ResizePty(ctx, &tasks.ResizePtyRequest{
		ContainerID: t.id,
		Width:       w,
		Height:      h,
	})
	return err
}

func (t *task) Checkpoint(ctx context.Context, opts ...CheckpointTaskOpts) (d v1.Descriptor, err error) {
	request := &tasks.CheckpointTaskRequest{
		ContainerID: t.id,
	}
	var i CheckpointTaskInfo
	for _, o := range opts {
		if err := o(&i); err != nil {
			return d, err
		}
	}
	request.ParentCheckpoint = i.ParentCheckpoint
	if i.Options != nil {
		any, err := typeurl.MarshalAny(i.Options)
		if err != nil {
			return d, err
		}
		request.Options = any
	}
	// make sure we pause it and resume after all other filesystem operations are completed
	if err := t.Pause(ctx); err != nil {
		return d, err
	}
	defer t.Resume(ctx)
	cr, err := t.client.ContainerService().Get(ctx, t.id)
	if err != nil {
		return d, err
	}
	var index v1.Index
	if err := t.checkpointTask(ctx, &index, request); err != nil {
		return d, err
	}
	if err := t.checkpointImage(ctx, &index, cr.Image); err != nil {
		return d, err
	}
	if err := t.checkpointRWSnapshot(ctx, &index, cr.Snapshotter, cr.RootFS); err != nil {
		return d, err
	}
	index.Annotations = make(map[string]string)
	index.Annotations["image.name"] = cr.Image
	return t.writeIndex(ctx, &index)
}

// UpdateTaskInfo allows updated specific settings to be changed on a task
type UpdateTaskInfo struct {
	// Resources updates a tasks resource constraints
	Resources interface{}
}

// UpdateTaskOpts allows a caller to update task settings
type UpdateTaskOpts func(context.Context, *Client, *UpdateTaskInfo) error

func (t *task) Update(ctx context.Context, opts ...UpdateTaskOpts) error {
	request := &tasks.UpdateTaskRequest{
		ContainerID: t.id,
	}
	var i UpdateTaskInfo
	for _, o := range opts {
		if err := o(ctx, t.client, &i); err != nil {
			return err
		}
	}
	if i.Resources != nil {
		any, err := typeurl.MarshalAny(i.Resources)
		if err != nil {
			return err
		}
		request.Resources = any
	}
	_, err := t.client.TaskService().Update(ctx, request)
	return err
}

func (t *task) checkpointTask(ctx context.Context, index *v1.Index, request *tasks.CheckpointTaskRequest) error {
	response, err := t.client.TaskService().Checkpoint(ctx, request)
	if err != nil {
		return err
	}
	// add the checkpoint descriptors to the index
	for _, d := range response.Descriptors {
		index.Manifests = append(index.Manifests, v1.Descriptor{
			MediaType: d.MediaType,
			Size:      d.Size_,
			Digest:    d.Digest,
			Platform: &v1.Platform{
				OS:           goruntime.GOOS,
				Architecture: goruntime.GOARCH,
			},
		})
	}
	return nil
}

func (t *task) checkpointRWSnapshot(ctx context.Context, index *v1.Index, snapshotterName string, id string) error {
	rw, err := rootfs.Diff(ctx, id, fmt.Sprintf("checkpoint-rw-%s", id), t.client.SnapshotService(snapshotterName), t.client.DiffService())
	if err != nil {
		return err
	}
	rw.Platform = &v1.Platform{
		OS:           goruntime.GOOS,
		Architecture: goruntime.GOARCH,
	}
	index.Manifests = append(index.Manifests, rw)
	return nil
}

func (t *task) checkpointImage(ctx context.Context, index *v1.Index, image string) error {
	if image == "" {
		return fmt.Errorf("cannot checkpoint image with empty name")
	}
	ir, err := t.client.ImageService().Get(ctx, image)
	if err != nil {
		return err
	}
	index.Manifests = append(index.Manifests, ir.Target)
	return nil
}

func (t *task) writeIndex(ctx context.Context, index *v1.Index) (v1.Descriptor, error) {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(index); err != nil {
		return v1.Descriptor{}, err
	}
	return writeContent(ctx, t.client.ContentStore(), v1.MediaTypeImageIndex, t.id, buf)
}

func writeContent(ctx context.Context, store content.Store, mediaType, ref string, r io.Reader) (d v1.Descriptor, err error) {
	writer, err := store.Writer(ctx, ref, 0, "")
	if err != nil {
		return d, err
	}
	defer writer.Close()
	size, err := io.Copy(writer, r)
	if err != nil {
		return d, err
	}
	if err := writer.Commit(size, ""); err != nil {
		return d, err
	}
	return v1.Descriptor{
		MediaType: mediaType,
		Digest:    writer.Digest(),
		Size:      size,
	}, nil
}
