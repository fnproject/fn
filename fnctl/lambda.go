// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	aws_lambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/docker/docker/pkg/jsonmessage"
	lambdaImpl "github.com/iron-io/lambda/lambda"
	"github.com/urfave/cli"
)

func init() {
	if len(runtimeCreateHandlers) != len(runtimeImportHandlers) {
		panic("incomplete implementation of runtime support")
	}

}

func lambda() cli.Command {
	var flags []cli.Flag

	flags = append(flags, getFlags()...)

	return cli.Command{
		Name:      "lambda",
		Usage:     "create and publish lambda functions",
		ArgsUsage: "fnclt lambda",
		Subcommands: []cli.Command{
			{
				Name:      "create-function",
				Usage:     `create Docker image that can run your Lambda function, where files are the contents of the zip file to be uploaded to AWS Lambda.`,
				ArgsUsage: "name runtime handler /path [/paths...]",
				Action:    create,
				Flags:     flags,
			},
			{
				Name:      "test-function",
				Usage:     `runs local dockerized Lambda function and writes output to stdout.`,
				ArgsUsage: "name [--payload <value>]",
				Action:    test,
				Flags:     flags,
			},
			{
				Name:      "aws-import",
				Usage:     `converts an existing Lambda function to an image, where the function code is downloaded to a directory in the current working directory that has the same name as the Lambda function.`,
				ArgsUsage: "arn region image/name [--profile <aws profile>] [--version <version>] [--download-only]",
				Action:    awsImport,
				Flags:     flags,
			},
		},
	}
}

func getFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "payload",
			Usage: "Payload to pass to the Lambda function. This is usually a JSON object.",
			Value: "{}",
		},
		cli.StringFlag{
			Name:  "version",
			Usage: "Version of the function to import.",
			Value: "$LATEST",
		},
		cli.BoolFlag{
			Name:  "download-only",
			Usage: "Only download the function into a directory. Will not create a Docker image.",
		},
	}
}

func create(c *cli.Context) error {
	args := c.Args()
	if len(args) < 4 {
		return fmt.Errorf("Expected at least 4 arguments, NAME RUNTIME HANDLER and file %d", len(args))
	}
	functionName := args[0]
	runtime := args[1]
	handler := args[2]
	fileNames := args[3:]

	files := make([]fileLike, 0, len(fileNames))
	opts := createImageOptions{
		Name:          functionName,
		Base:          fmt.Sprintf("iron/lambda-%s", runtime),
		Package:       "",
		Handler:       handler,
		OutputStream:  newdockerJSONWriter(os.Stdout),
		RawJSONStream: true,
	}

	if handler == "" {
		return errors.New("No handler specified.")
	}

	rh, ok := runtimeCreateHandlers[runtime]
	if !ok {
		return fmt.Errorf("unsupported runtime %v", runtime)
	}

	if err := rh(fileNames, &opts); err != nil {
		return err
	}

	for _, fileName := range fileNames {
		file, err := os.Open(fileName)
		if err != nil {
			return err
		}
		defer file.Close()
		files = append(files, file)
	}

	return createDockerfile(opts, files...)

}

var runtimeCreateHandlers = map[string]func(filenames []string, opts *createImageOptions) error{
	"nodejs":    func(filenames []string, opts *createImageOptions) error { return nil },
	"python2.7": func(filenames []string, opts *createImageOptions) error { return nil },
	"java8": func(filenames []string, opts *createImageOptions) error {
		if len(filenames) != 1 {
			return errors.New("Java Lambda functions can only include 1 file and it must be a JAR file.")
		}

		if filepath.Ext(filenames[0]) != ".jar" {
			return errors.New("Java Lambda function package must be a JAR file.")
		}

		opts.Package = filepath.Base(filenames[0])
		return nil
	},
}

func test(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return fmt.Errorf("Missing NAME argument")
	}
	functionName := args[0]

	exists, err := lambdaImpl.ImageExists(functionName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Function %s does not exist.", functionName)
	}

	payload := c.String("payload")
	// Redirect output to stdout.
	return lambdaImpl.RunImageWithPayload(functionName, payload)
}

func awsImport(c *cli.Context) error {
	args := c.Args()
	if len(args) < 3 {
		return fmt.Errorf("Missing arguments ARN, REGION and/or IMAGE")
	}

	version := c.String("version")
	downloadOnly := c.Bool("download-only")
	profile := c.String("profile")
	arn := args[0]
	region := args[1]
	image := args[2]

	function, err := getFunction(profile, region, version, arn)
	if err != nil {
		return err
	}
	functionName := *function.Configuration.FunctionName

	err = os.Mkdir(fmt.Sprintf("./%s", functionName), os.ModePerm)
	if err != nil {
		return err
	}

	tmpFileName, err := downloadToFile(*function.Code.Location)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFileName)

	if downloadOnly {
		// Since we are a command line program that will quit soon, it is OK to
		// let the OS clean `files` up.
		return err
	}

	opts := createImageOptions{
		Name:          functionName,
		Base:          fmt.Sprintf("iron/lambda-%s", *function.Configuration.Runtime),
		Package:       "",
		Handler:       *function.Configuration.Handler,
		OutputStream:  newdockerJSONWriter(os.Stdout),
		RawJSONStream: true,
	}

	runtime := *function.Configuration.Runtime
	rh, ok := runtimeImportHandlers[runtime]
	if !ok {
		return fmt.Errorf("unsupported runtime %v", runtime)
	}

	files, err := rh(functionName, tmpFileName, &opts)
	if err != nil {
		return nil
	}

	if image != "" {
		opts.Name = image
	}

	return createDockerfile(opts, files...)
}

var (
	runtimeImportHandlers = map[string]func(functionName, tmpFileName string, opts *createImageOptions) ([]fileLike, error){
		"nodejs":    basicImportHandler,
		"python2.7": basicImportHandler,
		"java8": func(functionName, tmpFileName string, opts *createImageOptions) ([]fileLike, error) {
			fmt.Println("Found Java Lambda function. Going to assume code is a single JAR file.")
			path := filepath.Join(functionName, "function.jar")
			if err := os.Rename(tmpFileName, path); err != nil {
				return nil, err
			}
			fd, err := os.Open(path)
			if err != nil {
				return nil, err
			}

			files := []fileLike{fd}
			opts.Package = filepath.Base(files[0].(*os.File).Name())
			return files, nil
		},
	}
)

func basicImportHandler(functionName, tmpFileName string, opts *createImageOptions) ([]fileLike, error) {
	return unzipAndGetTopLevelFiles(functionName, tmpFileName)
}

const fnYAMLTemplate = `
app: %s
image: %s
route: "/%s"
`

func createFunctionYaml(image string) error {
	strs := strings.Split(image, "/")
	data := []byte(fmt.Sprintf(fnYAMLTemplate, strs[0], image, strs[1]))
	return ioutil.WriteFile(filepath.Join(image, "function.yaml"), data, 0644)
}

type createImageOptions struct {
	Name          string
	Base          string
	Package       string // Used for Java, empty string for others.
	Handler       string
	OutputStream  io.Writer
	RawJSONStream bool
}

type fileLike interface {
	io.Reader
	Stat() (os.FileInfo, error)
}

var errNoFiles = errors.New("No files to add to image")

// Create a Dockerfile that adds each of the files to the base image. The
// expectation is that the base image sets up the current working directory
// inside the image correctly.  `handler` is set to be passed to node-lambda
// for now, but we may have to change this to accomodate other stacks.
func makeDockerfile(base string, pkg string, handler string, files ...fileLike) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "FROM %s\n", base)

	for _, file := range files {
		// FIXME(nikhil): Validate path, no parent paths etc.
		info, err := file.Stat()
		if err != nil {
			return buf.Bytes(), err
		}

		fmt.Fprintf(&buf, "ADD [\"%s\", \"./%s\"]\n", info.Name(), info.Name())
	}

	fmt.Fprint(&buf, "CMD [")
	if pkg != "" {
		fmt.Fprintf(&buf, "\"%s\", ", pkg)
	}
	// FIXME(nikhil): Validate handler.
	fmt.Fprintf(&buf, `"%s"`, handler)
	fmt.Fprint(&buf, "]\n")

	return buf.Bytes(), nil
}

// Creates a docker image called `name`, using `base` as the base image.
// `handler` is the runtime-specific name to use for a lambda invocation (i.e.
// <module>.<function> for nodejs). `files` should be a list of files+dirs
// *relative to the current directory* that are to be included in the image.
func createDockerfile(opts createImageOptions, files ...fileLike) error {
	if len(files) == 0 {
		return errNoFiles
	}

	df, err := makeDockerfile(opts.Base, opts.Package, opts.Handler, files...)
	if err != nil {
		return err
	}

	fmt.Printf("Creating directory: %s ... ", opts.Name)
	if err := os.MkdirAll(opts.Name, os.ModePerm); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Printf("Creating Dockerfile: %s ... ", filepath.Join(opts.Name, "Dockerfile"))
	outputFile, err := os.Create(filepath.Join(opts.Name, "Dockerfile"))
	if err != nil {
		return err
	}
	fmt.Println("OK")

	for _, f := range files {
		fstat, err := f.Stat()
		if err != nil {
			return err
		}
		fmt.Printf("Copying file: %s ... ", filepath.Join(opts.Name, fstat.Name()))
		src, err := os.Create(filepath.Join(opts.Name, fstat.Name()))
		if err != nil {
			return err
		}
		if _, err := io.Copy(src, f); err != nil {
			return err
		}
		fmt.Println("OK")
	}

	if _, err = outputFile.Write(df); err != nil {
		return err
	}

	fmt.Print("Creating function.yaml ... ")
	if err := createFunctionYaml(opts.Name); err != nil {
		return err
	}
	fmt.Println("OK")
	return nil
}

type dockerJSONWriter struct {
	under io.Writer
	w     io.Writer
}

func newdockerJSONWriter(under io.Writer) *dockerJSONWriter {
	r, w := io.Pipe()
	go func() {
		err := jsonmessage.DisplayJSONMessagesStream(r, under, 1, true, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()
	return &dockerJSONWriter{under, w}
}

func (djw *dockerJSONWriter) Write(p []byte) (int, error) {
	return djw.w.Write(p)
}

func downloadToFile(url string) (string, error) {
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

	if _, err := io.Copy(tmpFile, downloadResp.Body); err != nil {
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func unzipAndGetTopLevelFiles(dst, src string) (files []fileLike, topErr error) {
	files = make([]fileLike, 0)

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
			if err := os.Mkdir(path, 0644); err != nil {
				return nil, err
			}
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

			if _, topErr = io.Copy(fd, zipFd); topErr != nil {
				// OK to skip closing fd here.
				break
			}

			if err := zipFd.Close(); err != nil {
				return nil, err
			}

			// Only top-level files go into the list since that is what CreateImage expects.
			if filepath.Dir(f.Name) == "." {
				if _, topErr = fd.Seek(0, 0); topErr != nil {
					break
				}

				files = append(files, fd)
			} else {
				if err := fd.Close(); err != nil {
					return nil, err
				}
			}
		}
	}
	return
}

func getFunction(awsProfile, awsRegion, version, arn string) (*aws_lambda.GetFunctionOutput, error) {
	creds := credentials.NewChainCredentials([]credentials.Provider{
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{
			Filename: "", // Look in default location.
			Profile:  awsProfile,
		},
	})

	conf := aws.NewConfig().WithCredentials(creds).WithCredentialsChainVerboseErrors(true).WithRegion(awsRegion)
	sess := session.New(conf)
	conn := aws_lambda.New(sess)
	resp, err := conn.GetFunction(&aws_lambda.GetFunctionInput{
		FunctionName: aws.String(arn),
		Qualifier:    aws.String(version),
	})

	return resp, err
}
