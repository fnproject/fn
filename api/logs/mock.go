package logs

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"github.com/fnproject/fn/api/models"
)

type mock struct {
	Logs map[string][]byte
}

func NewMock() models.LogStore {
	return &mock{make(map[string][]byte)}
}

func (m *mock) InsertLog(ctx context.Context, appID, callID string, callLog io.Reader) error {
	bytes, err := ioutil.ReadAll(callLog)
	m.Logs[callID] = bytes
	return err
}

func (m *mock) GetLog(ctx context.Context, appID, callID string) (io.Reader, error) {
	logEntry, ok := m.Logs[callID]
	if !ok {
		return nil, models.ErrCallLogNotFound
	}
	return bytes.NewReader(logEntry), nil
}
