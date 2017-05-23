package main

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
	denvs := []string{}
	if c.String("log-level") != "" {
		denvs = append(denvs, "GIN_MODE="+c.String("log-level"))
	}
	// docker run --rm -it --name functions -v ${PWD}/data:/app/data -v /var/run/docker.sock:/var/run/docker.sock -p 8080:8080 treeder/functions
	wd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("Getwd failed")
	}
	args := []string{"run", "--rm", "-i",
		"--name", "functions",
		"-v", fmt.Sprintf("%s/data:/app/data", wd),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-p", "8080:8080",
	}
	for _, v := range denvs {
		args = append(args, "-e", v)
	}
	args = append(args, functionsDockerImage)
	cmd := exec.Command("docker", args...)
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
