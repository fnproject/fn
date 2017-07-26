package logs

import (
	"context"
	"github.com/pkg/errors"
	"github.com/fnproject/fn/api/models"
)

type mock struct {
	Logs map[string]*models.FnCallLog
	ds   models.Datastore
}

func NewMock() models.FnLog {
	return NewMockInit(nil)
}

func NewMockInit(logs map[string]*models.FnCallLog) models.FnLog {
	if logs == nil {
		logs = map[string]*models.FnCallLog{}
	}
	fnl := NewValidator(&mock{logs, nil})
	return fnl
}

func (m *mock) SetDatastore(ctx context.Context, ds models.Datastore) {
	m.ds = ds
}

func (m *mock) InsertLog(ctx context.Context, callID string, callLog string) error {
	m.Logs[callID] = &models.FnCallLog{CallID: callID, Log: callLog}
	return nil
}

func (m *mock) GetLog(ctx context.Context, callID string) (*models.FnCallLog, error) {
	logEntry := m.Logs[callID]
	if logEntry == nil {
		return nil, errors.New("Call log not found")
	}

	return m.Logs[callID], nil
}

func (m *mock) DeleteLog(ctx context.Context, callID string) error {
	delete(m.Logs, callID)
	return nil
}
