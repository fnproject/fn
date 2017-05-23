package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func updateCmd() cli.Command {
	return cli.Command{
		Name:   "update",
		Usage:  "pulls latest functions server",
		Action: update,
	}
}

func update(c *cli.Context) error {
	args := []string{"pull",
		"treeder/functions:latest",
	}
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
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
