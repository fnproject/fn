package metrics

import (
	"context"
	"io"

	"github.com/fnproject/fn/api/models"
	"go.opencensus.io/trace"
)

func NewLogstore(ls models.LogStore) models.LogStore {
	return &metricls{ls}
}

type metricls struct {
	ls models.LogStore
}

func (m *metricls) InsertCall(ctx context.Context, call *models.Call) error {
	ctx, span := trace.StartSpan(ctx, "ls_insert_call")
	defer span.End()
	return m.ls.InsertCall(ctx, call)
}

func (m *metricls) GetCall(ctx context.Context, appName, callID string) (*models.Call, error) {
	ctx, span := trace.StartSpan(ctx, "ls_get_call")
	defer span.End()
	return m.ls.GetCall(ctx, appName, callID)
}

func (m *metricls) GetCalls(ctx context.Context, filter *models.CallFilter) ([]*models.Call, error) {
	ctx, span := trace.StartSpan(ctx, "ls_get_calls")
	defer span.End()
	return m.ls.GetCalls(ctx, filter)
}

func (m *metricls) InsertLog(ctx context.Context, appName, callID string, callLog io.Reader) error {
	ctx, span := trace.StartSpan(ctx, "ls_insert_log")
	defer span.End()
	return m.ls.InsertLog(ctx, appName, callID, callLog)
}

func (m *metricls) GetLog(ctx context.Context, appName, callID string) (io.Reader, error) {
	ctx, span := trace.StartSpan(ctx, "ls_get_log")
	defer span.End()
	return m.ls.GetLog(ctx, appName, callID)
}

func (m *metricls) Close() error {
	return m.ls.Close()
}
