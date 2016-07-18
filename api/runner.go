package api

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/go/common"
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
	log.Infoln("HOST:", req.Host)
	rUrl := req.URL
	appName := rUrl.Query().Get("app")
	log.Infoln("app_name", appName, "path:", req.URL.Path)
	if appName != "" {
		// passed in the name
	} else {
		host := strings.Split(req.Host, ":")[0]
		appName = strings.Split(host, ".")[0]
		log.Infoln("app_name from host", appName)
	}

	app, err := getApp(appName)
	if err != nil {
		common.SendError(w, 400, fmt.Sprintln("This app does not exist. Please create app first.", err))
		return
	}
	log.Infoln("app", app)

	// find route
	for _, el := range app.Routes {
		// TODO: copy/use gorilla's pattern matching here
		if el.Path == req.URL.Path {
			// Boom, run it!
			err = checkAndPull(el.Image)
			if err != nil {
				common.SendError(w, 404, fmt.Sprintln("The image could not be pulled:", err))
				return
			}
			if el.Type == "app" {
				DockerHost(el, w)
				return
			} else { // "run"
				// TODO: timeout 59 seconds
				DockerRun(el, w, req)
				return
			}
		}
	}
	common.SendError(w, 404, fmt.Sprintln("The requested endpoint does not exist."))
}

// TODO: use Docker utils from docker-job for this and a few others in here
func DockerRun(route *Route3, w http.ResponseWriter, req *http.Request) {
	log.Infoln("route:", route)
	image := route.Image
	payload, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.WithError(err).Errorln("Error reading request body")
		return
	}
	log.WithField("payload", "---"+string(payload)+"---").Infoln("incoming request")
	log.WithField("image", image).Infoln("About to run using this image")

	// TODO: swap all this out with Titan's running via API
	cmd := exec.Command("docker", "run", "--rm", "-i", "-e", fmt.Sprintf("PAYLOAD=%v", string(payload)), image)
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
	var b bytes.Buffer
	buff := bufio.NewWriter(&b)

	go io.Copy(buff, stdout)
	go io.Copy(buff, stderr)

	log.Printf("Waiting for command to finish...")
	if err = cmd.Wait(); err != nil {
		// job failed
		log.Infoln("job finished with err:", err)
		log.WithFields(log.Fields{"metric": "run.errors", "value": 1, "type": "count"}).Infoln("failed run")
		// TODO: wrap error in json "error": buff
	} else {
		log.Infoln("Docker ran successfully:", b.String())
		// print
		log.WithFields(log.Fields{"metric": "run.success", "value": 1, "type": "count"}).Infoln("successful run")
	}
	log.WithFields(log.Fields{"metric": "run", "value": 1, "type": "count"}).Infoln("job ran")
	buff.Flush()
	if route.ContentType != "" {
		w.Header().Set("Content-Type", route.ContentType)
	}
	fmt.Fprintln(w, string(bytes.Trim(b.Bytes(), "\x00")))
}

func DockerHost(el *Route3, w http.ResponseWriter) {
	ra := runningImages[el.Image]
	if ra == nil {
		ra = &RunningApp{}
		ra.Route = el
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
			// TODO: What if the app fails to start? Don't want to keep starting the container
		}
	} else {
		// TODO: check if it's still running?
		// TODO: if ra.terminated, then start new container?
	}
	fmt.Println("RunningApp:", ra)
	// TODO: if connection fails, check if container still running?  If not, start it again
	resp, err := http.Get(fmt.Sprintf("http://0.0.0.0:%v%v", ra.Port, el.ContainerPath))
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
}

func checkAndPull(image string) error {
	err := execAndPrint("docker", []string{"inspect", image})
	if err != nil {
		// image does not exist, so let's pull
		fmt.Println("Image not found locally, will pull.", err)
		err = execAndPrint("docker", []string{"pull", image})
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
