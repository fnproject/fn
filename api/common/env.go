package common

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// GetEnv looks up a key under its name in env or name+_FILE to read the value
// from a file. fallback will be defaulted to if a value is not found.
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	} else if value, ok := os.LookupEnv(key + "_FILE"); ok {
		dat, err := ioutil.ReadFile(filepath.Clean(value))
		if err == nil {
			return string(dat)
		}
	}
	return fallback
}

// GetEnvInt looks up a key under its name in env or name+_FILE to read the
// value from a file. fallback will be defaulted to if a value is not found.
func GetEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		// linter liked this better than if/else
		var err error
		var i int
		if i, err = strconv.Atoi(value); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"string": value, "environment_key": key}).Fatal("Failed to convert string to int")
		}
		return i
	} else if value, ok := os.LookupEnv(key + "_FILE"); ok {
		dat, err := ioutil.ReadFile(filepath.Clean(value))
		if err == nil {
			var err error
			var i int
			if i, err = strconv.Atoi(strings.TrimSpace(string(dat))); err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{"string": dat, "environment_key": key}).Fatal("Failed to convert string to int")
			}
			return i
		}
	}
	return fallback
}

// GetEnvDuration looks up a key under its name in env or name+_FILE to read
// the value from a file. fallback will be defaulted to if a value is not
// found. if an integer is provided, the value will be returned in seconds
// (value * time.Second)
func GetEnvDuration(key string, fallback time.Duration) time.Duration {
	var err error
	res := fallback
	if tmp := os.Getenv(key); tmp != "" {
		// if the returned value is not null it needs to be either an integral value in seconds or a parsable duration-format string
		res, err = time.ParseDuration(tmp)
		if err != nil {
			// try to parse an int
			s, perr := strconv.Atoi(tmp)
			if perr != nil {
				logrus.WithError(err).WithFields(logrus.Fields{"duration_string": tmp, "environment_key": key}).Fatal("Failed to parse duration from env")
			} else {
				res = time.Duration(s) * time.Second
			}
		}
	}
	return res
}
