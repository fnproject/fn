package runner

import (
	"bufio"
	"fmt"
	"io"

	"context"
	"github.com/Sirupsen/logrus"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/runner/common"
)

type FuncLogger interface {
	Writer(ctx context.Context, appName, path, image, reqID string) io.Writer
}

// FuncLogger reads STDERR output from a container and outputs it in a parsed structured log format, see: https://github.com/treeder/functions/issues/76
type DefaultFuncLogger struct {
	logDB models.FnLog
}

func NewFuncLogger(logDB models.FnLog) FuncLogger {
	return &DefaultFuncLogger{logDB}
}

func (l *DefaultFuncLogger) persistLog(ctx context.Context, log logrus.FieldLogger, reqID, logText string) {
	err := l.logDB.InsertLog(ctx, reqID, logText)
	if err != nil {
		log.WithError(err).Println(fmt.Sprintf(
			"Unable to persist log for call %v. Error: %v", reqID, err))
	}
}

func (l *DefaultFuncLogger) Writer(ctx context.Context, appName, path, image, reqID string) io.Writer {
	r, w := io.Pipe()

	go func(reader io.Reader) {
		log := common.Logger(ctx)
		log = log.WithFields(logrus.Fields{"user_log": true, "app_name": appName,
			"path": path, "image": image, "call_id": reqID})

		var res string
		errMsg := "-------Unable to get full log, it's too big-------"
		fmt.Fscanf(reader, "%v", &res)
		if len(res) >= bufio.MaxScanTokenSize {
			res = res[0:bufio.MaxScanTokenSize - len(errMsg)] + errMsg
		}

		l.persistLog(ctx, log, reqID, res)
	}(r)
	return w
}
