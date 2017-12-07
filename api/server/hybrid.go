package server

import (
	"context"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/go-openapi/strfmt"
)

func (s *Server) handleRunnerEnqueue(c *gin.Context) {
	ctx := c.Request.Context()

	// TODO make this a list & let Push take a list!
	var call models.Call
	err := c.BindJSON(&call)
	if err != nil {
		handleErrorResponse(c, models.ErrInvalidJSON)
		return
	}

	// XXX (reed): validate the call struct

	// TODO/NOTE: if this endpoint is called multiple times for the same call we
	// need to figure out the behavior we want. as it stands, there will be N
	// messages for 1 call which only clogs up the MQ with spurious messages
	// (possibly useful if things get wedged, not the point), the task will still
	// just run once by the first runner to set it to status=running. we may well
	// want to push msg only if inserting the call fails, but then we have a call
	// in queued state with no message (much harder to handle). having this
	// endpoint be retry safe seems ideal and runners likely won't spam it, so current
	// behavior is okay [but beware of implications].
	call.Status = "queued"
	_, err = s.MQ.Push(ctx, &call)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	// at this point, the message is on the queue and could be picked up by a
	// runner and enter into 'running' state before we can insert it in the db as
	// 'queued' state. we can ignore any error inserting into db here and Start
	// will ensure the call exists in the db in 'running' state there.
	s.Datastore.InsertCall(ctx, &call)

	c.JSON(200, struct {
		M string `json:"msg"`
	}{M: "enqueued call"})
}

func (s *Server) handleRunnerDequeue(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// TODO finalize and move
	type m struct {
		AppName string `json:"app_name"`
		Path    string `json:"path"`
	}
	type resp struct {
		M []m `json:"calls"`
	}

	// long poll until ctx expires / we find a message
	var b common.Backoff
	for {
		msg, err := s.MQ.Reserve(ctx)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}
		if msg != nil {
			c.JSON(200, resp{M: []m{{AppName: msg.AppName, Path: msg.Path}}})
			return
		}

		b.Sleep(ctx)

		select {
		case <-ctx.Done():
			c.JSON(200, resp{M: make([]m, 0)})
			return
		default:
		}
	}
}

func (s *Server) handleRunnerStart(c *gin.Context) {
	var body struct {
		AppName string `json:"app_name"`
		CallID  string `json:"call_id"`
	}

	err := c.BindJSON(&body)
	if err != nil {
		handleErrorResponse(c, models.ErrInvalidJSON)
		return
	}

	// TODO hook up update. we really just want it to set status to running iff
	// status=queued, but this must be in a txn in Update with behavior:
	// queued->running
	// running->error (returning error)
	// error->error (returning error)
	// success->success (returning error)
	// timeout->timeout (returning error)
	//
	// there is nuance for running->error as in theory it could be the correct machine retrying
	// and we risk not running a task [ever]. needs further thought, but marking as error will
	// cover our tracks since if the db is down we can't run anything anyway (treat as such).
	var call models.Call
	call.AppName = body.AppName
	call.ID = body.CallID
	call.Status = "running"
	call.StartedAt = strfmt.DateTime(time.Now())
	//err := s.Datastore.UpdateCall(c.Request.Context(), &call)
	//if err != nil {
	//if err == InvalidStatusChange {
	//// TODO we could either let UpdateCall handle setting to error or do it
	//// here explicitly

	//if err := s.MQ.Delete(&call); err != nil { // TODO change this to take some string(s), not a whole call
	//logrus.WithFields(logrus.Fields{"id": call.Id}).WithError(err).Error("error deleting mq message")
	//// just log this one, return error from update call
	//}
	//}
	//handleErrorResponse(c, err)
	//return
	//}

	c.JSON(200, struct {
		M string `json:"msg"`
	}{M: "slingshot: engage"})
}

func (s *Server) handleRunnerFinish(c *gin.Context) {
	ctx := c.Request.Context()

	var body struct {
		Call models.Call `json:"call"`
		Log  string      `json:"log"` // TODO use multipart so that we don't have to serialize/deserialize this? measure..
	}
	err := c.BindJSON(&body)
	if err != nil {
		handleErrorResponse(c, models.ErrInvalidJSON)
		return
	}

	// TODO validate?
	call := body.Call

	// TODO this needs UpdateCall functionality to work for async and should only work if:
	// running->error|timeout|success
	// TODO all async will fail here :( all sync will work fine :) -- *feeling conflicted*
	if err := s.Datastore.InsertCall(ctx, &call); err != nil {
		common.Logger(ctx).WithError(err).Error("error inserting call into datastore")
		// note: Not returning err here since the job could have already finished successfully.
	}

	if err := s.LogDB.InsertLog(ctx, call.AppName, call.ID, strings.NewReader(body.Log)); err != nil {
		common.Logger(ctx).WithError(err).Error("error uploading log")
		// note: Not returning err here since the job could have already finished successfully.
	}

	// TODO we don't know whether a call is async or sync. we likely need an additional
	// arg in params for a message id and can detect based on this. for now, delete messages
	// for sync and async even though sync doesn't have any (ignore error)
	if err := s.MQ.Delete(ctx, &call); err != nil { // TODO change this to take some string(s), not a whole call
		common.Logger(ctx).WithError(err).Error("error deleting mq msg")
		// note: Not returning err here since the job could have already finished successfully.
	}

	c.JSON(200, struct {
		M string `json:"msg"`
	}{M: "good night, sweet prince"})
}
