package lb

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sirupsen/logrus"
)

var (
	ErrNoNodes        = errors.New("no nodes available")
	ErrUnknownCommand = errors.New("unknown command")
)

func sendValue(w http.ResponseWriter, v interface{}) {
	err := json.NewEncoder(w).Encode(v)

	if err != nil {
		logrus.WithError(err).Error("error writing response response")
	}
}

func sendSuccess(w http.ResponseWriter, msg string) {
	err := json.NewEncoder(w).Encode(struct {
		Msg string `json:"msg"`
	}{
		Msg: msg,
	})

	if err != nil {
		logrus.WithError(err).Error("error writing response response")
	}
}

func sendError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)

	err := json.NewEncoder(w).Encode(struct {
		Msg string `json:"msg"`
	}{
		Msg: msg,
	})

	if err != nil {
		logrus.WithError(err).Error("error writing response response")
	}
}

func NullHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendError(w, http.StatusNotFound, ErrUnknownCommand.Error())
	})
}
