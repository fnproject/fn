package completer

import (
	"github.com/spf13/viper"
	"github.com/Sirupsen/logrus"
	"github.com/fnproject/fn/api/server"
	"context"
	"strings"
	"github.com/fnproject/fn/api/runner/task"
	"net/http"
)

type completerFeature struct {
	completerUrl string
	key          string
}

type invalidCompletionRequest struct {
	message string
	code    int
}

func (e *invalidCompletionRequest) Code() int {
	return e.code
}

func (e *invalidCompletionRequest) Error() string {
	return e.message
}

const (
	EnvCompleterUrl   = "completer_url"
	EnvCompleterToken = "completer_token"
)

// SetupFromEnv Enables the fn completer for all routes that have FN_COMPLETER_ENABLED  set to true
// if the COMPLETER_URL env var is set it also filters calls that have
func SetupFromEnv(ctx context.Context, server *server.Server) {

	url := viper.GetString(EnvCompleterUrl)

	if url == "" {
		logrus.Info("Completer disabled")
		return
	}

	key := viper.GetString(EnvCompleterToken)
	secure := false
	if key != "" {
		secure = true
	}
	logrus.WithFields(logrus.Fields{"completer_url": url, "completer_security": secure}).Info("Completer enabled")

	feature := &completerFeature{
		completerUrl: url,
		key:          key,
	}

	server.AddTaskListener(feature)
	return
}

func findConfigVal(env map[string]string, key string) (string, string, bool) {
	cKey := canonKey(key)
	if v, ok := env[cKey]; ok {
		return cKey, v, ok
	}
	return "", "", false
}

func canonKey(headerName string) string {
	return strings.Replace(strings.ToUpper(headerName), "-", "_", -1)
}

func (c *completerFeature) BeforeTaskStart(ctx context.Context, task *task.Config) error {
	if _, v, ok := findConfigVal(task.Env, "FN_COMPLETER_ENABLED"); ok && strings.ToUpper(v) == "TRUE" {
		task.Env["FN_COMPLETER_BASE_URL"] = c.completerUrl

		// if threadID is set then this is (defacto) a completer call, require a token  if configured
		_, _, ok := findConfigVal(task.Env, "HEADER_FNPROJECT_THREADID")
		if !ok {
			// normal function invocation
			return nil
		}

		if c.key == "" {
			return nil
		}
		cookieKey, cookie, ok := findConfigVal(task.Env, "HEADER_FNPROJECT_COMPLETER_TOKEN")
		if !ok {
			return &invalidCompletionRequest{
				code:    http.StatusBadRequest,
				message: "Invalid completion call, token missing",
			}
		}

		if cookie != c.key {
			return &invalidCompletionRequest{
				code:    http.StatusBadRequest,
				message: "Invalid completion call, bad token",
			}
		}

		// hide the cookie from calls
		delete(task.Env, cookieKey)

	}

	return nil
}
