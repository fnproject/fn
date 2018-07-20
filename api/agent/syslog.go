package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/syslog"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"
	"go.opencensus.io/trace"
)

// syslogConns may return a non-nil io.WriteCloser and an error simultaneously,
// the error containing any errors from connecting to any of the syslog URLs, and the
// io.WriteCloser writing to any syslogURLs that were successfully connected to.
// the returned io.WriteCloser is a Writer to each conn, it should be wrapped in another
// writer that writes syslog formatted messages (by line).
func syslogConns(ctx context.Context, syslogURLs string) (io.WriteCloser, error) {
	// TODO(reed): we should likely add a trace per conn, need to plumb tagging better
	ctx, span := trace.StartSpan(ctx, "syslog_conns")
	defer span.End()

	if len(syslogURLs) == 0 {
		return nullReadWriter{}, nil
	}

	// gather all the conns, re-use the line we make in the syslogWriter
	// to write the same bytes to each of the conns.
	var conns []io.WriteCloser
	var errs []error

	sinks := strings.Split(syslogURLs, ",")
	for _, s := range sinks {
		conn, err := dialSyslog(ctx, strings.TrimSpace(s))
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to setup remote syslog connection to %v: %v", s, err))
			continue
		}

		conns = append(conns, conn)
	}

	// do this before checking length of conns
	var err error
	if len(errs) > 0 {
		for _, e := range errs {
			err = fmt.Errorf("%v%v, ", err, e)
		}
	}

	if len(conns) == 0 {
		return nullReadWriter{}, err
	}

	return multiWriteCloser(conns), err
}

func dialSyslog(ctx context.Context, syslogURL string) (io.WriteCloser, error) {
	url, err := url.Parse(syslogURL)
	if err != nil {
		return nil, err
	}

	common.Logger(ctx).WithField("syslog_url", url).Debug("dialing syslog url")

	var dialer net.Dialer
	deadline, ok := ctx.Deadline()
	if ok {
		dialer.Deadline = deadline
	}

	// slice off 'xxx://' and dial it
	switch url.Scheme {
	case "udp", "tcp":
		return dialer.Dial(url.Scheme, syslogURL[6:])
	case "tls":
		return tls.DialWithDialer(&dialer, "tcp", syslogURL[6:], nil)
	default:
		return nil, fmt.Errorf("Unsupported scheme, please use {tcp|udp|tls}: %s: ", url.Scheme)
	}
}

// syslogWriter prepends a syslog format with call-specific details
// for each data segment provided in Write(). This doesn't use
// log/syslog pkg because we do not need pid for every line (expensive),
// and we have a format that is easier to read than hiding in preamble.
// this writes logfmt formatted syslog with values for call, function, and
// app, it is up to the user to use logfmt from their functions to get a
// fully formatted line out.
// TODO not pressing, but we could support json & other formats, too, upon request.
type syslogWriter struct {
	pres  []byte
	post  []byte
	b     *bytes.Buffer
	clock func() time.Time

	// the syslog conns (presumably)
	io.Writer
}

const severityMask = 0x07
const facilityMask = 0xf8

func newSyslogWriter(callID, fnID, appID string, severity syslog.Priority, wc io.Writer, buf *bytes.Buffer) *syslogWriter {
	// Facility = LOG_USER
	pr := (syslog.LOG_USER & facilityMask) | (severity & severityMask)

	// <priority>VERSION ISOTIMESTAMP HOSTNAME APPLICATION PID      MESSAGEID STRUCTURED-DATA MSG
	//
	// and for us:
	// <22>2             ISOTIMESTAMP fn       appID       funcName callID    -               MSG
	// ex:
	//<11>2 2018-02-31T07:42:21Z Fn - - - -  call_id=123 func_name=rdallman/yodawg app_id=123 loggo hereo

	// TODO we could use json for structured data and do that whole thing. up to whoever.
	return &syslogWriter{
		pres:   []byte(fmt.Sprintf(`<%d>2`, pr)),
		post:   []byte(fmt.Sprintf(`fn - - - - call_id=%s fn_id=%s app_id=%s `, callID, fnID, appID)),
		b:      buf,
		Writer: wc,
		clock:  time.Now,
	}
}

func (sw *syslogWriter) Write(p []byte) (int, error) {
	// re-use buffer to write in timestamp hodge podge and reduce writes to
	// the conn by buffering a whole line here before writing to conn.

	buf := sw.b
	buf.Reset()
	buf.Write(sw.pres)
	buf.WriteString(" ")
	buf.WriteString(sw.clock().UTC().Format(time.RFC3339))
	buf.WriteString(" ")
	buf.Write(sw.post)
	buf.Write(p)
	n, err := io.Copy(sw.Writer, buf)
	return int(n), err
}
