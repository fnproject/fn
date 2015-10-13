package main

import (
	"bufio"
	"bytes"
	"fmt"
	// "github.com/gorilla/mux"
	"github.com/iron-io/go/common"
	"github.com/iron-io/golog"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
)

type RunningApp struct {
	Route         *Route3
	Port          int
	ContainerName string
}

var runningImages map[string]*RunningApp

func init() {
	runningImages = make(map[string]*RunningApp)
	fmt.Println("ENV:", os.Environ())
}

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
			if el.Type == "app" {
				ra := runningImages[el.Image]
				if ra == nil {
					ra = &RunningApp{}
					ra.Route = &el
					ra.Port = rand.Intn(9999-9000) + 9000
					ra.ContainerName = fmt.Sprintf("c_%v", rand.Intn(10000))
					runningImages[el.Image] = ra
					// TODO: timeout 59 minutes. Mark it in ra as terminated.
					cmd := exec.Command("docker", "run", "--name", ra.ContainerName, "--rm", "-i", "-p", fmt.Sprintf("%v:8080", ra.Port), el.Image)
					// TODO: What should we do with the output here?  Store it? Send it to a log service?
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					// TODO: Need to catch interrupt and stop all containers that are started, see devo/dj for how to do this
					if err := cmd.Start(); err != nil {
						log.Fatal(err)
					}
					// TODO: What if the app fails to start? Don't want to keep starting the container
				} else {
					// TODO: check if it's still running?
					// TODO: if ra.terminated, then start new container?
				}
				fmt.Println("RunningApp:", ra)
				// TODO: if connection fails, check if container still running?  If not, start it again
				resp, err := http.Get(fmt.Sprintf("http://localhost:%v%v", ra.Port, el.ContainerPath))
				if err != nil {
					common.SendError(w, 404, fmt.Sprintln("The requested app endpoint does not exist.", err))
					return
				}
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					common.SendError(w, 500, fmt.Sprintln("Error reading response body", err))
					return
				}
				fmt.Fprintln(w, string(body))
				return
			} else { // "run"
				// TODO: timeout 59 seconds
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
	}
	common.SendError(w, 404, fmt.Sprintln("The requested endpoint does not exist."))
}

func checkAndPull(route *Route3) error {
	err := execAndPrint("docker", []string{"inspect", route.Image})
	if err != nil {
		// image does not exist, so let's pull
		fmt.Println("Image not found locally, will pull.", err)
		err = execAndPrint("docker", []string{"pull", route.Image})
	}
	return err
}

func execAndPrint(cmdstr string, args []string) error {
	var bout bytes.Buffer
	buffout := bufio.NewWriter(&bout)
	var berr bytes.Buffer
	bufferr := bufio.NewWriter(&berr)
	cmd := exec.Command(cmdstr, args...)
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

	log.Printf("Waiting for cmd to finish...")
	err = cmd.Wait()
	fmt.Println("stderr:", berr.String())
	fmt.Println("stdout:", bout.String())
	return err
}
