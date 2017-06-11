package worker

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"os"
)

var (
	TaskDir     string
	envFlag     string
	payloadFlag string
	TaskId      string
	configFlag  string
)

// call this to parse flags before using the other methods.
func ParseFlags() {
	flag.StringVar(&TaskDir, "d", "", "task dir")
	flag.StringVar(&envFlag, "e", "", "environment type")
	flag.StringVar(&payloadFlag, "payload", "", "payload file")
	flag.StringVar(&TaskId, "id", "", "task id")
	flag.StringVar(&configFlag, "config", "", "config file")
	flag.Parse()
	if os.Getenv("TASK_ID") != "" {
		TaskId = os.Getenv("TASK_ID")
	}
	if os.Getenv("TASK_DIR") != "" {
		TaskDir = os.Getenv("TASK_DIR")
	}
	if os.Getenv("PAYLOAD_FILE") != "" {
		payloadFlag = os.Getenv("PAYLOAD_FILE")
	}
	if os.Getenv("CONFIG_FILE") != "" {
		configFlag = os.Getenv("CONFIG_FILE")
	}
}

func PayloadReader() (io.ReadCloser, error) {
	return os.Open(payloadFlag)
}

func PayloadFromJSON(v interface{}) error {
	reader, err := PayloadReader()
	if err != nil {
		return err
	}
	defer reader.Close()
	return json.NewDecoder(reader).Decode(v)
}

func PayloadAsString() (string, error) {
	reader, err := PayloadReader()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ConfigReader() (io.ReadCloser, error) {
	return os.Open(configFlag)
}

func ConfigFromJSON(v interface{}) error {
	reader, err := ConfigReader()
	if err != nil {
		return err
	}
	defer reader.Close()
	return json.NewDecoder(reader).Decode(v)
}

func ConfigAsString() (string, error) {
	reader, err := ConfigReader()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func IronTaskId() string {
	return TaskId
}

func IronTaskDir() string {
	return TaskDir
}
