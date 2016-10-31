package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/iron-io/iron_go3/config"
	lambdaImpl "github.com/iron-io/lambda/lambda"
	"github.com/urfave/cli"
)

var availableRuntimes = []string{"nodejs", "python2.7", "java8"}

type lambdaCmd struct {
	settings  config.Settings
	token     *string
	projectID *string
}

type lambdaCreateCmd struct {
	lambdaCmd

	functionName string
	runtime      string
	handler      string
	fileNames    []string
}

func (lcc *lambdaCreateCmd) Config() error {
	return nil
}

type DockerJsonWriter struct {
	under io.Writer
	w     io.Writer
}

func NewDockerJsonWriter(under io.Writer) *DockerJsonWriter {
	r, w := io.Pipe()
	go func() {
		err := jsonmessage.DisplayJSONMessagesStream(r, under, 1, true, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()
	return &DockerJsonWriter{under, w}
}

func (djw *DockerJsonWriter) Write(p []byte) (int, error) {
	return djw.w.Write(p)
}

func (lcc *lambdaCreateCmd) run(c *cli.Context) error {

	handler := c.String("handler")
	functionName := c.String("name")
	runtime := c.String("runtime")

	lcc.fileNames = c.Args()
	lcc.handler = handler
	lcc.functionName = functionName
	lcc.runtime = runtime

	files := make([]lambdaImpl.FileLike, 0, len(lcc.fileNames))
	opts := lambdaImpl.CreateImageOptions{
		Name:          lcc.functionName,
		Base:          fmt.Sprintf("iron/lambda-%s", lcc.runtime),
		Package:       "",
		Handler:       lcc.handler,
		OutputStream:  NewDockerJsonWriter(os.Stdout),
		RawJSONStream: true,
	}

	if lcc.handler == "" {
		return errors.New("No handler specified.")
	}

	// For Java we allow only 1 file and it MUST be a JAR.
	if lcc.runtime == "java8" {
		if len(lcc.fileNames) != 1 {
			return errors.New("Java Lambda functions can only include 1 file and it must be a JAR file.")
		}

		if filepath.Ext(lcc.fileNames[0]) != ".jar" {
			return errors.New("Java Lambda function package must be a JAR file.")
		}

		opts.Package = filepath.Base(lcc.fileNames[0])
	}

	for _, fileName := range lcc.fileNames {
		file, err := os.Open(fileName)
		defer file.Close()
		if err != nil {
			return err
		}
		files = append(files, file)
	}

	return lambdaImpl.CreateImage(opts, files...)
}

func (lcc *lambdaCreateCmd) getFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:        "function-name",
			Usage:       "Name of function. This is usually follows Docker image naming conventions.",
			Destination: &lcc.functionName,
		},
		cli.StringFlag{
			Name:        "runtime",
			Usage:       fmt.Sprintf("Runtime that your Lambda function depends on. Valid values are %s.", strings.Join(availableRuntimes, ", ")),
			Destination: &lcc.runtime,
		},
		cli.StringFlag{
			Name:        "handler",
			Usage:       "function/class that is the entrypoint for this function. Of the form <module name>.<function name> for nodejs/Python, <full class name>::<function name base> for Java.",
			Destination: &lcc.handler,
		},
	}
}

func lambda() cli.Command {
	lcc := lambdaCreateCmd{}
	var flags []cli.Flag

	flags = append(flags, lcc.getFlags()...)
	return cli.Command{
		Name:      "lambda",
		Usage:     "create and publish lambda functions",
		ArgsUsage: "fnclt lambda",
		Subcommands: []cli.Command{
			{
				Name:      "create-function",
				Usage:     `Create Docker image that can run your Lambda function. The files are the contents of the zip file to be uploaded to AWS Lambda.`,
				ArgsUsage: "--function-name NAME --runtime RUNTIME --handler HANDLER file [files...]",
				Action:    lcc.run,
				Flags:     flags,
			},
		},
	}
}
