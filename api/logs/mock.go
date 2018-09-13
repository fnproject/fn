package logs

import (
	"bytes"
	"context"
	"encoding/base64"
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

func (m *mock) InsertLog(ctx context.Context, call *models.Call, callLog io.Reader) error {
	bytes, err := ioutil.ReadAll(callLog)
	m.Logs[call.ID] = bytes
	return err
}

func (m *mock) GetLog1(ctx context.Context, appID, callID string) (io.Reader, error) {
	logEntry, ok := m.Logs[callID]
	if !ok {
		return nil, models.ErrCallLogNotFound
	}
	return bytes.NewReader(logEntry), nil
}

func (m *mock) GetLog(ctx context.Context, fnID, callID string) (io.Reader, error) {
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

func (m *mock) GetCall1(ctx context.Context, appID, callID string) (*models.Call, error) {
	for _, t := range m.Calls {
		if t.ID == callID && t.AppID == appID {
			return t, nil
		}
	}

	return nil, models.ErrCallNotFound
}

func (m *mock) GetCall(ctx context.Context, fnID, callID string) (*models.Call, error) {
	for _, t := range m.Calls {
		if t.ID == callID &&
			t.FnID == fnID {
			return t, nil
		}
	}

	return nil, models.ErrCallNotFound
}

type sortC []*models.Call

func (s sortC) Len() int           { return len(s) }
func (s sortC) Less(i, j int) bool { return strings.Compare(s[i].ID, s[j].ID) < 0 }
func (s sortC) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetCalls1(ctx context.Context, filter *models.CallFilter) ([]*models.Call, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	// calls are in DESC order so use sort.Reverse
	sort.Sort(sort.Reverse(sortC(m.Calls)))

	var calls []*models.Call
	for _, c := range m.Calls {
		if len(calls) == filter.PerPage {
			break
		}

		if (filter.AppID == "" || c.AppID == filter.AppID) &&
			(filter.Path == "" || filter.Path == c.Path) &&
			(time.Time(filter.FromTime).IsZero() || time.Time(filter.FromTime).Before(time.Time(c.CreatedAt))) &&
			(time.Time(filter.ToTime).IsZero() || time.Time(c.CreatedAt).Before(time.Time(filter.ToTime))) &&
			(filter.Cursor == "" || strings.Compare(filter.Cursor, c.ID) > 0) {

			calls = append(calls, c)
		}
	}

	return calls, nil
}

func (m *mock) GetCalls(ctx context.Context, filter *models.CallFilter) (*models.CallList, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	// calls are in DESC order so use sort.Reverse
	sort.Sort(sort.Reverse(sortC(m.Calls)))

	var calls []*models.Call

	var cursor = ""
	if filter.Cursor != "" {
		s, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return nil, err
		}
		cursor = string(s)
	}

	for _, c := range m.Calls {
		if filter.PerPage > 0 && len(calls) == filter.PerPage {
			break
		}

		if (cursor == "" || strings.Compare(cursor, c.ID) > 0) &&
			(filter.FnID == "" || c.FnID == filter.FnID) &&
			(time.Time(filter.FromTime).IsZero() || time.Time(filter.FromTime).Before(time.Time(c.CreatedAt))) &&
			(time.Time(filter.ToTime).IsZero() || time.Time(c.CreatedAt).Before(time.Time(filter.ToTime))) {

			calls = append(calls, c)
		}
	}

	var nextCursor string
	if len(calls) > 0 && len(calls) == filter.PerPage {
		last := []byte(calls[len(calls)-1].ID)
		nextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	return &models.CallList{
		NextCursor: nextCursor,
		Items:      calls,
	}, nil
}

func (m *mock) Close() error {
	return nil
}
