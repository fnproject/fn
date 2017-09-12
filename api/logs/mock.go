package logs

import (
	"context"
	"github.com/fnproject/fn/api/models"
	"github.com/pkg/errors"
)

type mock struct {
	Logs map[string]*models.CallLog
	ds   models.Datastore
}

func NewMock() models.LogStore {
	return NewMockInit(nil)
}

func NewMockInit(logs map[string]*models.CallLog) models.LogStore {
	if logs == nil {
		logs = map[string]*models.CallLog{}
	}
	fnl := &mock{logs, nil}
	return fnl
}

func (m *mock) SetDatastore(ctx context.Context, ds models.Datastore) {
	m.ds = ds
}

func (m *mock) InsertLog(ctx context.Context, appName, callID, callLog string) error {
	m.Logs[callID] = &models.CallLog{CallID: callID, Log: callLog}
	return nil
}

func (m *mock) GetLog(ctx context.Context, appName, callID string) (*models.CallLog, error) {
	logEntry := m.Logs[callID]
	if logEntry == nil {
		return nil, errors.New("Call log not found")
	}

	return m.Logs[callID], nil
}

func (m *mock) DeleteLog(ctx context.Context, appName, callID string) error {
	delete(m.Logs, callID)
	return nil
}
