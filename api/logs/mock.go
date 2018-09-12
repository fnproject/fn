package logs

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/fnproject/fn/api/models"
)

type mock struct {
	Logs      map[string][]byte
	Calls     []*models.Call
	CallsList *models.CallList
}

func NewMock(args ...interface{}) models.LogStore {
	var mocker mock
	for _, a := range args {
		switch x := a.(type) {
		case []*models.Call:
			mocker.Calls = x
		case *models.CallList:
			mocker.CallsList = x
		default:
			panic("unknown type handed to mocker. add me")
		}
	}
	mocker.Logs = make(map[string][]byte)
	return &mocker
}

func (m *mock) InsertLog(ctx context.Context, appID, fnID, callID string, callLog io.Reader) error {
	bytes, err := ioutil.ReadAll(callLog)
	m.Logs[callID] = bytes
	return err
}

func (m *mock) GetLog(ctx context.Context, callID string) (io.Reader, error) {
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

func (m *mock) GetCall(ctx context.Context, callID string) (*models.Call, error) {
	for _, t := range m.Calls {
		if t.ID == callID {
			return t, nil
		}
	}

	return nil, models.ErrCallNotFound
}

type sortC []*models.Call

func (s sortC) Len() int           { return len(s) }
func (s sortC) Less(i, j int) bool { return strings.Compare(s[i].ID, s[j].ID) < 0 }
func (s sortC) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetCalls(ctx context.Context, filter *models.CallFilter) (*models.CallList, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	// calls are in DESC order so use sort.Reverse
	sort.Sort(sort.Reverse(sortC(m.Calls)))

	var calls *models.CallList
	// for i, c := range m.CallsList {
	// 	if len(calls) == filter.PerPage {
	// 		break
	// 	}

	// 	if (filter.AppID == "" || c.Items[i].AppID == filter.AppID) &&
	// 		(time.Time(filter.FromTime).IsZero() || time.Time(filter.FromTime).Before(time.Time(c.Items[i].CreatedAt))) &&
	// 		(time.Time(filter.ToTime).IsZero() || time.Time(c.Items[i].CreatedAt).Before(time.Time(filter.ToTime))) {
	// 		// (filter.Cursor == "" || strings.Compare(filter.Cursor, c.Items[i].Annotations) > 0)

	// 		calls = append(calls, c)
	// 	}
	// }

	return calls, nil
}

func (m *mock) Close() error {
	return nil
}
