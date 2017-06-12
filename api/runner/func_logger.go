package runner

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/Sirupsen/logrus"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/runner/common"
)

type FuncLogger interface {
	Writer(ctx context.Context, appName, path, image, reqID string) io.WriteCloser
}

// FuncLogger reads STDERR output from a container and outputs it in a parsed structured log format, see: https://github.com/treeder/functions/issues/76
type DefaultFuncLogger struct {
	logDB models.FnLog
}

func NewFuncLogger(logDB models.FnLog) FuncLogger {
	return &DefaultFuncLogger{logDB}
}

type writer struct {
	bytes.Buffer

	db      models.FnLog
	ctx     context.Context
	reqID   string
	appName string
	image   string
	path    string
}

func (w *writer) Close() error {
	return w.db.InsertLog(context.TODO(), w.reqID, w.String())
}

func (w *writer) Write(b []byte) (int, error) {
	n, err := w.Buffer.Write(b)

	// for now, also write to stderr so we can debug quick ;)
	// TODO this should be a separate FuncLogger but time is running short !
	//log := common.Logger(w.ctx)
	//log = log.WithFields(logrus.Fields{"user_log": true, "app_name": w.appName,
	//"path": w.path, "image": w.image, "call_id": w.reqID})
	//log.Println(string(b))

	return n, err
}

// overrides Write, keeps Close
type limitWriter struct {
	n, max int
	io.WriteCloser
}

func newLimitWriter(max int, w io.WriteCloser) io.WriteCloser {
	return &limitWriter{max: max, WriteCloser: w}
}

func (l *limitWriter) Write(b []byte) (int, error) {
	if l.n > l.max {
		return 0, errors.New("max log size exceeded, truncating log")
	}
	n, err := l.WriteCloser.Write(b)
	l.n += n
	return n, err
}

func (l *DefaultFuncLogger) Writer(ctx context.Context, appName, path, image, reqID string) io.WriteCloser {
	const MB = 1 * 1024 * 1024
	return newLimitWriter(MB, &writer{
		db:      l.logDB,
		ctx:     ctx,
		appName: appName,
		path:    path,
		image:   image,
		reqID:   reqID,
	})
}
