package main

/*
usage: fn init <name>

If there's a Dockerfile found, this will generate the basic file with just the image name. exit
It will then try to decipher the runtime based on the files in the current directory, if it can't figure it out, it will ask.
It will then take a best guess for what the entrypoint will be based on the language, it it can't guess, it will ask.

*/

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
