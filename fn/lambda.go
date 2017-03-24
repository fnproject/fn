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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	aws_lambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
)

var runtimes = map[string]string{
	"nodejs4.3": "lambda-nodejs4.3",
}

func lambda() cli.Command {
	var flags []cli.Flag

	flags = append(flags, getFlags()...)

	return cli.Command{
		Name:  "lambda",
		Usage: "create and publish lambda functions",
		Subcommands: []cli.Command{
			{
				Name:      "aws-import",
				Usage:     `converts an existing Lambda function to an image, where the function code is downloaded to a directory in the current working directory that has the same name as the Lambda function.`,
				ArgsUsage: "<arn> <region> <image/name>",
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
		cli.StringSliceFlag{
			Name:  "config",
			Usage: "function configuration",
		},
	}
}

func transcribeEnvConfig(configs []string) map[string]string {
	c := make(map[string]string)
	for _, v := range configs {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) == 1 {
			// TODO: Make sure it is compatible cross platform
			c[kv[0]] = fmt.Sprintf("$%s", kv[0])
		} else {
			c[kv[0]] = kv[1]
		}
	}
	return c
}

func awsImport(c *cli.Context) error {
	args := c.Args()

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
		Base:          runtimes[(*function.Configuration.Runtime)],
		Package:       "",
		Handler:       *function.Configuration.Handler,
		OutputStream:  newdockerJSONWriter(os.Stdout),
		RawJSONStream: true,
		Config:        transcribeEnvConfig(c.StringSlice("config")),
	}

	runtime := *function.Configuration.Runtime
	rh, ok := runtimeImportHandlers[runtime]
	if !ok {
		return fmt.Errorf("unsupported runtime %v", runtime)
	}

	_, err = rh(functionName, tmpFileName, &opts)
	if err != nil {
		return nil
	}

	if image != "" {
		opts.Name = image
	}

	fmt.Print("Creating func.yaml ... ")
	if err := createFunctionYaml(opts, functionName); err != nil {
		return err
	}
	fmt.Println("OK")

	return nil
}

var (
	runtimeImportHandlers = map[string]func(functionName, tmpFileName string, opts *createImageOptions) ([]fileLike, error){
		"nodejs4.3": basicImportHandler,
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

func createFunctionYaml(opts createImageOptions, functionName string) error {
	strs := strings.Split(opts.Name, "/")
	path := fmt.Sprintf("/%s", strs[1])

	funcDesc := &funcfile{
		Name:    opts.Name,
		Path:    &path,
		Config:  opts.Config,
		Version: "0.0.1",
		Runtime: &opts.Base,
		Cmd:     opts.Handler,
	}

	out, err := yaml.Marshal(funcDesc)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(functionName, "func.yaml"), out, 0644)
}

type createImageOptions struct {
	Name          string
	Base          string
	Package       string // Used for Java, empty string for others.
	Handler       string
	OutputStream  io.Writer
	RawJSONStream bool
	Config        map[string]string
}

type fileLike interface {
	io.Reader
	Stat() (os.FileInfo, error)
}

var errNoFiles = errors.New("No files to add to image")

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
