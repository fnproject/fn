package lb

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"
	"time"

	"fmt"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

const K8sGrouperDriver = "kubernetes"

func NewK8sClient(conf Config) (*kubernetes.Clientset, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(k8sConfig)
}

// NewK8sGrouper returns a Grouper that will return the entire list of nodes
// that match a particular Kubernetes pattern and that pass a healthcheck.
// Healthchecks happen at a specified interval.
// Each fnlb instance mainains its own list of healthy nodes.
// An endpoint is exposed for listing nodes.
func NewK8sGrouper(conf Config, clientset *kubernetes.Clientset) (Grouper, error) {
	// Sanity-check this up-front.
	_, err := labels.Parse(conf.LabelSelector)
	if err != nil {
		return nil, err
	}

	k := &k8sGrouper{
		nodeList:        make(map[string]nodeState),
		nodeHealthyList: make([]string, 0),

		// for k8s watching
		k8sClient:  clientset,
		targetPort: conf.TargetPort,

		// XXX (per reed): need to be reconfigurable at some point
		hcInterval:    time.Duration(conf.HealthcheckInterval) * time.Second,
		hcEndpoint:    conf.HealthcheckEndpoint,
		hcUnhealthy:   int64(conf.HealthcheckUnhealthy),
		hcHealthy:     int64(conf.HealthcheckHealthy),
		hcTimeout:     time.Duration(conf.HealthcheckTimeout) * time.Second,
		minAPIVersion: *conf.MinAPIVersion,

		// for health checks
		httpClient: &http.Client{Transport: conf.Transport},
	}

	go k.watchForPods(conf.Namespace, conf.LabelSelector)
	go k.healthcheck()
	return k, nil
}

// k8sGrouper will return all healthy nodes it is tracking that
// kubernetes has told it about.
// Each k8sGrouper maintains an independent list of watch nodes from
// the kube master.
// Sort considerations are as for allGrouper.
type k8sGrouper struct {

	// health checker state and lock
	nodeLock        sync.RWMutex
	nodeList        map[string]nodeState
	nodeHealthyList []string

	k8sClient  *kubernetes.Clientset
	targetPort int

	httpClient *http.Client

	hcInterval    time.Duration
	hcEndpoint    string
	hcUnhealthy   int64
	hcHealthy     int64
	hcTimeout     time.Duration
	minAPIVersion semver.Version
}

// TODO (jang): many of these are near-identical with the allGrouper implementations. Factor them out.

func (k *k8sGrouper) add(newb string, address string) error {
	k.nodeLock.Lock()
	defer k.nodeLock.Unlock()
	if node, ok := k.nodeList[newb]; ok {
		if node.address != address {
			logrus.WithField("node", node).WithField("address", address).Info("Updating address of registered node")
		} else {
			logrus.WithField("node", node).WithField("address", address).Debug("Attempt to add extant node at known address")
			return fmt.Errorf("Attempt to add extant node %s", newb)
		}
	}

	k.nodeList[newb] = nodeState{healthy: StateUnknown, address: address}
	return nil
}

func (k *k8sGrouper) remove(dead string) error {
	k.nodeLock.Lock()
	defer k.nodeLock.Unlock()
	if node, ok := k.nodeList[dead]; !ok {
		logrus.WithFields(logrus.Fields{"node": node}).Warn("Attempt to delete nonexistent node")
		return fmt.Errorf("Attempt to delete nonexistent node %s", dead)
	} else {
		logrus.WithField("node", node).WithField("address", node.address).Info("Removing registered node")
	}

	delete(k.nodeList, dead)
	return nil
}

func (k *k8sGrouper) publishHealth() {
	k.nodeLock.Lock()
	defer k.nodeLock.Unlock()

	// get a list of healthy nodes
	newList := make([]string, 0, len(k.nodeList))
	for _, value := range k.nodeList {
		if value.healthy == StateHealthy {
			newList = append(newList, value.address)
		}
	}

	// sort and update healthy List
	sort.Strings(newList)
	k.nodeHealthyList = newList
}

// return a copy
func (k *k8sGrouper) List(string) ([]string, error) {
	k.nodeLock.RLock()
	defer k.nodeLock.RUnlock()

	ret := make([]string, len(k.nodeHealthyList))
	copy(ret, k.nodeHealthyList)
	return ret, nil
}

func (k *k8sGrouper) runHealthCheck() {
	// Use the list we're maintaining from k8s
	k.nodeLock.RLock()
	defer k.nodeLock.RUnlock()

	nodes := k.nodeList
	for key, n := range nodes {
		if n.address != "" {
			go k.ping(key, n.address)
		}
	}
}

func (k *k8sGrouper) healthcheck() {
	// run hc immediately upon startup
	k.runHealthCheck()

	for range time.Tick(k.hcInterval) {
		k.runHealthCheck()
	}
}

func (k *k8sGrouper) getVersion(urlString string) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, urlString, nil)
	ctx, cancel := context.WithTimeout(context.Background(), k.hcTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	var v fnVersion
	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return "", err
	}
	return v.Version, nil
}

func (k *k8sGrouper) checkAPIVersion(node string) error {
	versionURL := "http://" + node + k.hcEndpoint

	version, err := k.getVersion(versionURL)
	if err != nil {
		return err
	}

	nodeVer, err := semver.NewVersion(version)
	if err != nil {
		return err
	}

	if nodeVer.LessThan(k.minAPIVersion) {
		return fmt.Errorf("incompatible API version: %v", nodeVer)
	}
	return nil
}

func (k *k8sGrouper) ping(node string, address string) {
	err := k.checkAPIVersion(address)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"node": node}).Error("Unable to check API version")
		k.fail(node)
	} else {
		k.alive(node)
	}
}

func (k *k8sGrouper) fail(key string) {
	isChanged := false

	k.nodeLock.Lock()

	// if deleted, skip
	node, ok := k.nodeList[key]
	if !ok {
		k.nodeLock.Unlock()
		return
	}

	node.success = 0
	node.fail++

	// overflow case
	if node.fail == 0 {
		node.fail = uint64(k.hcUnhealthy)
	}

	if (node.healthy == StateHealthy && node.fail >= uint64(k.hcUnhealthy)) || node.healthy == StateUnknown {
		node.healthy = StateUnhealthy
		isChanged = true
	}

	k.nodeList[key] = node
	k.nodeLock.Unlock()

	if isChanged {
		logrus.WithFields(logrus.Fields{"node": key}).Info("is unhealthy")
		k.publishHealth()
	}
}

func (k *k8sGrouper) alive(key string) {
	isChanged := false

	k.nodeLock.Lock()

	// if deleted, skip
	node, ok := k.nodeList[key]
	if !ok {
		k.nodeLock.Unlock()
		return
	}

	node.fail = 0
	node.success++

	// overflow case
	if node.success == 0 {
		node.success = uint64(k.hcHealthy)
	}

	if (node.healthy == StateUnhealthy && node.success >= uint64(k.hcHealthy)) || node.healthy == StateUnknown {
		node.healthy = StateHealthy
		isChanged = true
	}

	k.nodeList[key] = node
	k.nodeLock.Unlock()

	if isChanged {
		logrus.WithFields(logrus.Fields{"node": key}).Info("is healthy")
		k.publishHealth()
	}
}

func (k *k8sGrouper) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1/lb/nodes":
			switch r.Method {
			case "GET":
				k.listNodes(w, r)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (k *k8sGrouper) listNodes(w http.ResponseWriter, r *http.Request) {

	k.nodeLock.RLock()

	out := make(map[string]string, len(k.nodeList))

	for key, value := range k.nodeList {
		if value.healthy == StateHealthy {
			out[key] = "online"
		} else {
			out[key] = "offline"
		}
	}

	k.nodeLock.RUnlock()

	sendValue(w, struct {
		Nodes map[string]string `json:"nodes"`
	}{
		Nodes: out,
	})
}

func (k *k8sGrouper) watchForPods(namespace string, labelSelector string) {
	if namespace == "" {
		// Use our current namespace
		ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/" + v1.ServiceAccountNamespaceKey)
		if err != nil {
			// If we're not running inside k8s, bail out
			panic(err)
		}
		namespace = string(ns)
	}
	pods, err := k.k8sClient.CoreV1().Pods(namespace).Watch(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		panic(err.Error())
	}

	// This runs forever.
	for {
		select {
		case event := <-pods.ResultChan():
			if pod, ok := event.Object.(*v1.Pod); ok {
				logrus.
					WithField("Event", event.Type).
					WithField("Pod", pod.SelfLink).
					WithField("PodPhase", pod.Status.Phase).
					WithField("Address", pod.Status.PodIP).
					Debug("Change detected")
				switch event.Type {
				case watch.Added:
					fallthrough
				case watch.Modified:
					logrus.WithField("Pod", pod.SelfLink).WithField("PodIp", pod.Status.PodIP).Debug("New pod detected")
					address := ""
					if pod.Status.PodIP != "" && pod.Status.Phase == v1.PodRunning {
						address = fmt.Sprintf("%s:%d", pod.Status.PodIP, k.targetPort)
					}
					k.add(pod.SelfLink, address)
				case watch.Deleted:
					logrus.WithField("Pod", pod.SelfLink).WithField("PodIp", pod.Status.PodIP).Debug("Pod removed")
					k.remove(pod.SelfLink)
				}
			}
		}
	}
}
