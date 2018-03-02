package logs

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/fnproject/fn/api/models"
)

type mock struct {
	Logs  map[string][]byte
	Calls []*models.Call
}

func NewMock() models.LogStore {
	return &mock{Logs: make(map[string][]byte)}
}

func (m *mock) InsertLog(ctx context.Context, appName, callID string, callLog io.Reader) error {
	bytes, err := ioutil.ReadAll(callLog)
	m.Logs[callID] = bytes
	return err
}

func (m *mock) GetLog(ctx context.Context, appName, callID string) (io.Reader, error) {
	logEntry, ok := m.Logs[callID]
	if !ok {
		return nil, models.ErrCallLogNotFound
	}
	return bytes.NewReader(logEntry), nil
}

func (m *mock) InsertCall(ctx context.Context, call *models.Call) error {
	m.Calls = append(m.Calls, call)
	return nil
}

func (m *mock) GetCall(ctx context.Context, appName, callID string) (*models.Call, error) {
	for _, t := range m.Calls {
		if t.ID == callID && t.AppName == appName {
			return t, nil
		}
	}

	return nil, models.ErrCallNotFound
}

type sortC []*models.Call

func (s sortC) Len() int           { return len(s) }
func (s sortC) Less(i, j int) bool { return strings.Compare(s[i].ID, s[j].ID) < 0 }
func (s sortC) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetCalls(ctx context.Context, filter *models.CallFilter) ([]*models.Call, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	// calls are in DESC order so use sort.Reverse
	sort.Sort(sort.Reverse(sortC(m.Calls)))

	var calls []*models.Call
	for _, c := range m.Calls {
		if len(calls) == filter.PerPage {
			break
		}

		if (filter.AppName == "" || c.AppName == filter.AppName) &&
			(filter.Path == "" || filter.Path == c.Path) &&
			(time.Time(filter.FromTime).IsZero() || time.Time(filter.FromTime).Before(time.Time(c.CreatedAt))) &&
			(time.Time(filter.ToTime).IsZero() || time.Time(c.CreatedAt).Before(time.Time(filter.ToTime))) &&
			(filter.Cursor == "" || strings.Compare(filter.Cursor, c.ID) > 0) {

			calls = append(calls, c)
		}
	}

	return calls, nil
}
