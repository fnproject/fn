package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

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
	// Socket mount: docker run --rm -it --name functions -v ${PWD}/data:/app/data -v /var/run/docker.sock:/var/run/docker.sock -p 8080:8080 funcy/functions
	// OR dind: docker run --rm -it --name functions -v ${PWD}/data:/app/data --privileged -p 8080:8080 funcy/functions
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("Getwd failed:", err)
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
		log.Fatalln("starting command failed:", err)
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
		log.Println("interrupt caught, exiting")
		err = cmd.Process.Kill()
		if err != nil {
			log.Println("error: could not kill process:", err)
		}
	case err := <-done:
		if err != nil {
			log.Println("error: processed finished with error", err)
		} else {
			log.Println("process finished gracefully without error")
		}
	}
	return nil
}
