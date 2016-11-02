package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/iron-io/iron_go3/config"
	lambdaImpl "github.com/iron-io/lambda/lambda"
	"github.com/urfave/cli"

	"github.com/aws/aws-sdk-go/aws"
	aws_credentials "github.com/aws/aws-sdk-go/aws/credentials"
	aws_session "github.com/aws/aws-sdk-go/aws/session"
	aws_lambda "github.com/aws/aws-sdk-go/service/lambda"
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
	payload      string
	clientConext string
	arn          string
	version      string
	downloadOnly bool
	awsProfile   string
	image        string
	awsRegion    string
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
		cli.StringFlag{
			Name:        "payload",
			Usage:       "Payload to pass to the Lambda function. This is usually a JSON object.",
			Destination: &lcc.payload,
			Value:       "{}",
		},
		cli.StringFlag{
			Name:        "client-context",
			Usage:       "",
			Destination: &lcc.clientConext,
		},

		cli.StringFlag{
			Name:        "image",
			Usage:       "By default the name of the Docker image is the name of the Lambda function. Use this to set a custom name.",
			Destination: &lcc.image,
		},

		cli.StringFlag{
			Name:        "version",
			Usage:       "Version of the function to import.",
			Destination: &lcc.version,
		},

		cli.BoolFlag{
			Name:        "download-only",
			Usage:       "Only download the function into a directory. Will not create a Docker image.",
			Destination: &lcc.downloadOnly,
		},

		cli.StringFlag{
			Name:        "profile",
			Usage:       "AWS Profile to load from credentials file.",
			Destination: &lcc.awsProfile,
		},

		cli.StringFlag{
			Name:        "region",
			Usage:       "AWS region to use.",
			Value:       "us-east-1",
			Destination: &lcc.awsRegion,
		},
	}
}

func (lcc *lambdaCreateCmd) downloadToFile(url string) (string, error) {
	downloadResp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer downloadResp.Body.Close()

	// zip reader needs ReaderAt, hence the indirection.
	tmpFile, err := ioutil.TempFile("", "lambda-function-")
	if err != nil {
		return "", err
	}

	io.Copy(tmpFile, downloadResp.Body)
	tmpFile.Close()
	return tmpFile.Name(), nil
}

func (lcc *lambdaCreateCmd) unzipAndGetTopLevelFiles(dst, src string) (files []lambdaImpl.FileLike, topErr error) {
	files = make([]lambdaImpl.FileLike, 0)

	zipReader, err := zip.OpenReader(src)
	if err != nil {
		return files, err
	}
	defer zipReader.Close()

	var fd *os.File
	for _, f := range zipReader.File {
		path := filepath.Join(dst, f.Name)
		fmt.Printf("Extracting '%s' to '%s'\n", f.Name, path)
		if f.FileInfo().IsDir() {
			os.Mkdir(path, 0644)
			// Only top-level dirs go into the list since that is what CreateImage expects.
			if filepath.Dir(f.Name) == filepath.Base(f.Name) {
				fd, topErr = os.Open(path)
				if topErr != nil {
					break
				}
				files = append(files, fd)
			}
		} else {
			// We do not close fd here since we may want to use it to dockerize.
			fd, topErr = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
			if topErr != nil {
				break
			}

			var zipFd io.ReadCloser
			zipFd, topErr = f.Open()
			if topErr != nil {
				break
			}

			_, topErr = io.Copy(fd, zipFd)
			if topErr != nil {
				// OK to skip closing fd here.
				break
			}

			zipFd.Close()

			// Only top-level files go into the list since that is what CreateImage expects.
			if filepath.Dir(f.Name) == "." {
				_, topErr = fd.Seek(0, 0)
				if topErr != nil {
					break
				}

				files = append(files, fd)
			} else {
				fd.Close()
			}
		}
	}
	return
}

func (lcc *lambdaCreateCmd) getFunction() (*aws_lambda.GetFunctionOutput, error) {
	creds := aws_credentials.NewChainCredentials([]aws_credentials.Provider{
		&aws_credentials.EnvProvider{},
		&aws_credentials.SharedCredentialsProvider{
			Filename: "", // Look in default location.
			Profile:  lcc.awsProfile,
		},
	})

	conf := aws.NewConfig().WithCredentials(creds).WithCredentialsChainVerboseErrors(true).WithRegion(lcc.awsRegion)
	sess := aws_session.New(conf)
	conn := aws_lambda.New(sess)
	resp, err := conn.GetFunction(&aws_lambda.GetFunctionInput{
		FunctionName: aws.String(lcc.arn),
		Qualifier:    aws.String(lcc.version),
	})

	return resp, err
}

func (lcc *lambdaCreateCmd) init(c *cli.Context) {
	handler := c.String("handler")
	functionName := c.String("function-name")
	runtime := c.String("runtime")
	clientContext := c.String("client-context")
	payload := c.String("payload")
	version := c.String("version")
	downloadOnly := c.Bool("download-only")
	image := c.String("image")
	profile := c.String("profile")
	region := c.String("region")

	lcc.fileNames = c.Args()
	lcc.handler = handler
	lcc.functionName = functionName
	lcc.runtime = runtime
	lcc.clientConext = clientContext
	lcc.payload = payload
	lcc.version = version
	lcc.downloadOnly = downloadOnly
	lcc.awsProfile = profile
	lcc.image = image
	lcc.awsRegion = region
}

func (lcc *lambdaCreateCmd) create(c *cli.Context) error {
	lcc.init(c)

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

func (lcc *lambdaCreateCmd) runTest(c *cli.Context) error {
	lcc.init(c)
	exists, err := lambdaImpl.ImageExists(lcc.functionName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Function %s does not exist.", lcc.functionName)
	}

	// Redirect output to stdout.
	return lambdaImpl.RunImageWithPayload(lcc.functionName, lcc.payload)
}

func (lcc *lambdaCreateCmd) awsImport(c *cli.Context) error {
	lcc.init(c)
	function, err := lcc.getFunction()
	if err != nil {
		return err
	}
	functionName := *function.Configuration.FunctionName

	err = os.Mkdir(fmt.Sprintf("./%s", functionName), os.ModePerm)
	if err != nil {
		return err
	}

	tmpFileName, err := lcc.downloadToFile(*function.Code.Location)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFileName)

	var files []lambdaImpl.FileLike

	if *function.Configuration.Runtime == "java8" {
		fmt.Println("Found Java Lambda function. Going to assume code is a single JAR file.")
		path := filepath.Join(functionName, "function.jar")
		os.Rename(tmpFileName, path)
		fd, err := os.Open(path)
		if err != nil {
			return err
		}

		files = append(files, fd)
	} else {
		files, err = lcc.unzipAndGetTopLevelFiles(functionName, tmpFileName)
		if err != nil {
			return err
		}
	}

	if lcc.downloadOnly {
		// Since we are a command line program that will quit soon, it is OK to
		// let the OS clean `files` up.
		return err
	}

	opts := lambdaImpl.CreateImageOptions{
		Name:          functionName,
		Base:          fmt.Sprintf("iron/lambda-%s", *function.Configuration.Runtime),
		Package:       "",
		Handler:       *function.Configuration.Handler,
		OutputStream:  NewDockerJsonWriter(os.Stdout),
		RawJSONStream: true,
	}

	if lcc.image != "" {
		opts.Name = lcc.image
	}

	if *function.Configuration.Runtime == "java8" {
		opts.Package = filepath.Base(files[0].(*os.File).Name())
	}

	err = lambdaImpl.CreateImage(opts, files...)
	if err != nil {
		return err
	}
	return nil
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
				Action:    lcc.create,
				Flags:     flags,
			},
			{
				Name:      "test-function",
				Usage:     `Runs local Dockerized Lambda function and writes output to stdout.`,
				ArgsUsage: "--function-name NAME [--client-context <value>] [--payload <value>]",
				Action:    lcc.runTest,
				Flags:     flags,
			},
			{
				Name:      "aws-import",
				Usage:     `Converts an existing Lambda function to an image. The function code is downloaded to a directory in the current working directory that has the same name as the Lambda function..`,
				ArgsUsage: "[--region <region>] [--profile <aws profile>] [--version <version>] [--download-only] [--image <name>] ARN",
				Action:    lcc.awsImport,
				Flags:     flags,
			},
		},
	}
}
