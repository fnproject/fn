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

func NewMock(args ...interface{}) models.LogStore {
	var mocker mock
	for _, a := range args {
		switch x := a.(type) {
		case []*models.Call:
			mocker.Calls = x
		default:
			panic("unknown type handed to mocker. add me")
		}
	}
	mocker.Logs = make(map[string][]byte)
	return &mocker
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

func (m *mock) InsertCall(ctx context.Context, call *models.Call) error {
	m.Calls = append(m.Calls, call)
	return nil
}

func (m *mock) GetCall(ctx context.Context, appID, callID string) (*models.Call, error) {
	for _, t := range m.Calls {
		if t.ID == callID && t.AppID == appID {
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

		// TODO Invoke : filter by function ID and trigger ID
		if (filter.AppID == "" || c.AppID == filter.AppID) &&
			//(filter.Path == "" || filter.Path == c.Path) &&
			(time.Time(filter.FromTime).IsZero() || time.Time(filter.FromTime).Before(time.Time(c.CreatedAt))) &&
			(time.Time(filter.ToTime).IsZero() || time.Time(c.CreatedAt).Before(time.Time(filter.ToTime))) &&
			(filter.Cursor == "" || strings.Compare(filter.Cursor, c.ID) > 0) {

			calls = append(calls, c)
		}
	}

	return calls, nil
}

func (m *mock) Close() error {
	return nil
}
