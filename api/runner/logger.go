// This reads STDERR output from a container and outputs it in a parseable structured log format, see: https://github.com/iron-io/functions/issues/76
package runner

import (
	"bufio"
	"io"

	"github.com/Sirupsen/logrus"
)

type FuncLogger struct {
	r io.Reader
	w io.Writer
}

func NewFuncLogger(appName, path, function, requestID string) io.Writer {
	r, w := io.Pipe()
	funcLogger := &FuncLogger{
		r: r,
		w: w,
	}
	log := logrus.WithFields(logrus.Fields{"app_name": appName, "path": path, "function": function, "request_id": requestID})
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			log.Println(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.WithError(err).Println("There was an error with the scanner in attached container")
		}
	}(r)
	return funcLogger
}

func (l *FuncLogger) Write(p []byte) (n int, err error) {
	return l.w.Write(p)
}
