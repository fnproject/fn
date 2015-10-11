package main

import (
	"bufio"
	"bytes"
	"fmt"
	// "github.com/gorilla/mux"
	"github.com/iron-io/go/common"
	"github.com/iron-io/golog"
	"io"
	"log"
	"net/http"
	"os/exec"
)

func Run(w http.ResponseWriter, req *http.Request) {
	fmt.Println("RUN!!!!")
	// vars := mux.Vars(req)
	appName := req.FormValue("app")
	golog.Infoln("app_name", appName, "path:", req.URL.Path)

	app, err := getApp(appName)
	if err != nil {
		common.SendError(w, 400, fmt.Sprintln("This app does not exist. Please create app first.", err))
		return
	}
	golog.Infoln("app", app)

	var b bytes.Buffer
	buff := bufio.NewWriter(&b)

	// find route
	for _, el := range app.Routes {
		// TODO: copy/use gorilla's pattern matching here
		if el.Path == req.URL.Path {
			// Boom, run it!
			err = checkAndPull(&el)
			if err != nil {
				common.SendError(w, 404, fmt.Sprintln("The image could not be pulled:", err))
				return
			}
			cmd := exec.Command("docker", "run", "--rm", "-i", el.Image)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				log.Fatal(err)
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				log.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				log.Fatal(err)
			}
			go io.Copy(buff, stdout)
			go io.Copy(buff, stderr)

			log.Printf("Waiting for command to finish...")
			if err := cmd.Wait(); err != nil {
				log.Fatal(err)
			}
			log.Printf("Command finished with error: %v", err)
			buff.Flush()
			golog.Infoln("Docker ran successfully:", b.String())
			fmt.Fprintln(w, b.String())
			return
		}
	}
	common.SendError(w, 404, fmt.Sprintln("The requested endpoint does not exist."))
}

func checkAndPull(route *Route3) error {
	var bout bytes.Buffer
	buffout := bufio.NewWriter(&bout)
	var berr bytes.Buffer
	bufferr := bufio.NewWriter(&berr)
	cmd := exec.Command("docker", "inspect", route.Image)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go io.Copy(buffout, stdout)
	go io.Copy(bufferr, stderr)

	log.Printf("Waiting for docker inspect to finish...")
	err = cmd.Wait()
	fmt.Println("stderr:", berr.String())
	fmt.Println("stdout:", bout.String())
	return err
}
