package server

func (s *Server) handleRunnerEnqueue(c *gin.Context) {
	ctx := c.Request.Context()

	var call models.Call
	err := c.BindJSON(&call)
	if err != nil {
		handleErrorResponse(c, models.ErrInvalidJSON)
		return
	}

	// XXX (reed): validate the call struct

	call.State = "queued"

	_, err := s.MQ.Push(ctx, &call)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	// at this point, the message is on the queue and could be picked up by a
	// runner and enter into 'running' state before we can insert it in the db as
	// 'queued' state. we can ignore any error inserting into db here and Start
	// will ensure the call exists in the db in 'running' state there.
	db.InsertCall(ctx, call)
}

func (s *Server) handleRunnerDequeue(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var b common.Backoff
	for {
		msg, err := s.MQ.Reserve(ctx)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}
		if msg != nil {
			c.JSON([]struct {
				AppName string `json:"app_name"`
				Path    string `json:"path"`
			}{{AppName: msg.AppName, Path: msg.Path}})
			return
		}

		b.Sleep(ctx)

		switch {
		case <-ctx.Done():
			c.JSON([]struct{}{})
			return
		default:
		}
	}
}

func (s *Server) handleRunnerStart(c *gin.Context) {
}

func (s *Server) handleRunnerStart(c *gin.Context) {
}
