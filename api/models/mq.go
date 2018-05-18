package models

import (
	"context"
	"io"
)

// Message Queue is used to impose a total ordering on jobs that it will
// execute in order. calls are added to the queue via the Push() interface. The
// MQ must support a reserve-delete 2 step dequeue to allow implementing
// timeouts and retries.
//
// The Reserve() operation must return a job based on this total ordering
// (described below). At this point, the MQ backend must start a timeout on the
// job. If Delete() is not called on the call within the timeout, the call should
// be restored to the queue.
//
// Total ordering: The queue should maintain an ordering based on priority and
// logical time.  Priorities are currently 0-2 and available in the call's
// priority field.  call with higher priority always get pulled off the queue
// first.  Within the same priority, jobs should be available in FIFO order.

// When a job is required to be restored to the queue, it should maintain it's
// approximate order in the queue. That is, for jobs [A, B, C], with A being
// the head of the queue:
// Reserve() leads to A being passed to a consumer, and timeout started.
// Next Reserve() leads to B being dequeued. This consumer finishes running the
// call, leading to Delete() being called. B is now permanently erased from the
// queue.
// A's timeout occurs before the job is finished. At this point the ordering
// should be [A, C] and not [C, A].
type MessageQueue interface {
	// Push a call onto the queue. If any error is returned, the call SHOULD not be
	// queued. Note that this does not completely avoid double queueing, that is
	// OK, a check against the datastore will be performed after a dequeue.
	//
	// If the job's Delay value is > 0, the job should NOT be enqueued. The job
	// should only be available in the queue after at least Delay seconds have
	// elapsed. No ordering is required among multiple jobs queued with similar
	// delays. That is, if jobs {A, C} are queued at t seconds, both with Delay
	// = 5 seconds, and the same priority, then they may be available on the
	// queue as [C, A] or [A, C].
	Push(context.Context, *Call) (*Call, error)

	// Remove a job from the front of the queue, reserve it for a timeout and
	// return it. MQ implementations MUST NOT lose jobs in case of errors. That
	// is, in case of reservation failure, it should be possible to retrieve the
	// job on a future reservation.
	Reserve(context.Context) (*Call, error)

	// If a reservation is pending, consider it acknowledged and delete it. If
	// the job does not have an outstanding reservation, error. If a job did not
	// exist, succeed.
	Delete(context.Context, *Call) error

	// Close will close any underlying connections as needed.
	// Close is not safe to be called from multiple threads.
	io.Closer
}
