package main

/*
usage: fn init <name>

If there's a Dockerfile found, this will generate the basic file with just the image name. exit
It will then try to decipher the runtime based on the files in the current directory, if it can't figure it out, it will ask.
It will then take a best guess for what the entrypoint will be based on the language, it it can't guess, it will ask.

*/

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func startCmd() cli.Command {
	return cli.Command{
		Name:   "start",
		Usage:  "start a functions server",
		Action: start,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "log-level",
				Usage: "--log-level DEBUG to enable debugging",
			},
		},
	}
}

func start(c *cli.Context) error {
	denvs := ""
	if c.String("log-level") != "" {
		denvs += "-e GIN_MODE=" + c.String("log-level")
	}
	// docker run --rm -it --name functions -v ${PWD}/data:/app/data -v /var/run/docker.sock:/var/run/docker.sock -p 8080:8080 treeder/functions
	wd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("Getwd failed")
	}
	cmd := exec.Command("docker", "run", "--rm", "-i",
		"--name", "functions",
		"-v", fmt.Sprintf("%s/data:/app/data", wd),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-p", "8080:8080",
		"treeder/functions")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		logrus.WithError(err).Fatalln("starting command failed")
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	// catch ctrl-c and kill
	sigC := make(chan os.Signal, 2)
	signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigC:
		logrus.Infoln("interrupt caught, exiting")
		err = cmd.Process.Kill()
		if err != nil {
			logrus.WithError(err).Errorln("Could not kill process")
		}
	case err := <-done:
		if err != nil {
			logrus.WithError(err).Errorln("processed finished with error")
		} else {
			logrus.Println("process done gracefully without error")
		}
	}
	return nil
}
