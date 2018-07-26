package agent

import (
	"bytes"
	"log/syslog"
	"testing"
	"time"
)

func TestSyslogFormat(t *testing.T) {
	var b1 bytes.Buffer
	var b2 bytes.Buffer

	call := "12345"
	fnID := "myFnId"
	appID := "myAppId"
	now := time.Date(1982, 6, 25, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	writer := newSyslogWriter(call, fnID, appID, syslog.LOG_ERR, &nopCloser{&b1}, &b2)
	writer.clock = clock
	writer.Write([]byte("yo"))

	gold := `<11>2 1982-06-25T12:00:00Z fn - - - - call_id=12345 fn_id=myFnId app_id=myAppId yo`

	if b1.String() != gold {
		t.Fatalf("syslog was not what we expected: , wanted `%s`, got `%s` ", gold, b1.String())
	}
}
