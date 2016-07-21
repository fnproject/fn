package runner

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

type RunningApp struct {
	Route         *models.Route
	Port          int
	ContainerName string
}

var (
	ErrRunnerRouteNotFound = errors.New("Route not found on that application")
)

var runningImages map[string]*RunningApp

func init() {
	runningImages = make(map[string]*RunningApp)
	fmt.Println("ENV:", os.Environ())
}

func Run(c *gin.Context) error {
	log := c.MustGet("log").(logrus.FieldLogger)
	store := c.MustGet("store").(models.Datastore)

	appName := c.Param("app")

	if appName == "" {
		host := strings.Split(c.Request.Host, ":")[0]
		appName = strings.Split(host, ".")[0]
	}

	filter := &models.RouteFilter{
		AppName: appName,
	}

	routes, err := store.GetRoutes(filter)
	if err != nil {
		return err
	}

	route := c.Param("route")

	log.WithFields(logrus.Fields{"app": appName}).Debug("Running app")

	for _, el := range routes {
		if el.Path == route {
			err = checkAndPull(el.Image)
			if err != nil {
				return err
			}
			if el.Type == "app" {
				return DockerHost(el, c)
			} else {
				return DockerRun(el, c)
			}
		}
	}

	return ErrRunnerRouteNotFound
}

// TODO: use Docker utils from docker-job for this and a few others in here
func DockerRun(route *models.Route, c *gin.Context) error {
	image := route.Image
	payload, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	// log.WithField("payload", "---"+string(payload)+"---").Infoln("incoming request")
	// log.WithField("image", image).Infoln("About to run using this image")

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
		// log.Infoln("job finished with err:", err)
		// log.WithFields(log.Fields{"metric": "run.errors", "value": 1, "type": "count"}).Infoln("failed run")
		return err
		// TODO: wrap error in json "error": buff
	}

	// log.Infoln("Docker ran successfully:", b.String())
	// print
	// log.WithFields(log.Fields{"metric": "run.success", "value": 1, "type": "count"}).Infoln("successful run")
	// log.WithFields(log.Fields{"metric": "run", "value": 1, "type": "count"}).Infoln("job ran")
	buff.Flush()

	c.Data(http.StatusOK, "", bytes.Trim(b.Bytes(), "\x00"))

	return nil
}

func DockerHost(el *models.Route, c *gin.Context) error {
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
		// cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// TODO: Need to catch interrupt and stop all containers that are started, see devo/dj for how to do this
		if err := cmd.Start(); err != nil {
			return err
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
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	c.Data(http.StatusOK, "", body)
	return nil
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
	if berr.Len() != 0 {
		fmt.Println("stderr:", berr.String())
	}
	fmt.Println("stdout:", bout.String())
	return err
}
