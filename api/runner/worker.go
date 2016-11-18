package runner

import (
	"context"
	"sync"

	"github.com/iron-io/runner/drivers"
)

type TaskRequest struct {
	Ctx      context.Context
	Config   *Config
	Response chan TaskResponse
}

type TaskResponse struct {
	Result drivers.RunResult
	Err    error
}

// StartWorkers handle incoming tasks and spawns self-regulating container
// workers.
func StartWorkers(ctx context.Context, rnr *Runner, tasks <-chan TaskRequest) {
	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case task := <-tasks:
			wg.Add(1)
			go func(task TaskRequest) {
				defer wg.Done()
				result, err := rnr.Run(task.Ctx, task.Config)
				select {
				case task.Response <- TaskResponse{result, err}:
					close(task.Response)
				default:
				}
			}(task)
		}
	}

}

func RunTask(tasks chan TaskRequest, ctx context.Context, cfg *Config) (drivers.RunResult, error) {
	tresp := make(chan TaskResponse)
	treq := TaskRequest{Ctx: ctx, Config: cfg, Response: tresp}
	tasks <- treq
	resp := <-treq.Response
	return resp.Result, resp.Err
}
