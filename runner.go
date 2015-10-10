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
			// buff := bufio.NewWriter()
			go io.Copy(buff, stdout) //  <---- commented out because we will print out with buff.Scan()
			go io.Copy(buff, stderr)

			log.Printf("Waiting for command to finish...")
			if err := cmd.Wait(); err != nil {
				log.Fatal(err)
			}
			log.Printf("Command finished with error: %v", err)
			buff.Flush()
		}
	}
	golog.Infoln("Docker ran successfully:", b.String())
	fmt.Fprintln(w, b.String())
}
