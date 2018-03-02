package k8s

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync"

	"github.com/fnproject/fn/api/agent"
	agent_grpc "github.com/fnproject/fn/api/agent/nodepool/grpc"
	"github.com/fnproject/fn/poolmanager"
	pool_grpc "github.com/fnproject/fn/poolmanager/grpc"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var _ poolmanager.NodePoolManager = &k8sNPM{}

func NewK8sClient() (*kubernetes.Clientset, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(k8sConfig)
}

func NewK8sPool(ns string, labelSelector string, targetPort int, cert string, key string, ca string) (agent.NodePool, error) {
	client, err := NewK8sClient()
	if err != nil {
		return nil, err
	}

	if ns == "" {
		ns, err = namespace()
		if err != nil {
			return nil, err
		}
	}

	_, err = labels.Parse(labelSelector)
	if err != nil {
		return nil, err
	}

	k := &k8sNPM{
		client: client,

		cancel: make(chan struct{}),

		nodes: map[string]string{},
	}

	go k.watch(ns, labelSelector, targetPort)

	return agent_grpc.NewgRPCNodePool(cert, key, ca,
		k,
		NullCapacityAdvertiser,
		agent_grpc.GRPCRunnerFactory), nil
}

var NullCapacityAdvertiser poolmanager.CapacityAdvertiser = &nullCapacityAdvertiser{}

type nullCapacityAdvertiser struct{}

func (*nullCapacityAdvertiser) AssignCapacity(request *poolmanager.CapacityRequest)  {}
func (*nullCapacityAdvertiser) ReleaseCapacity(request *poolmanager.CapacityRequest) {}
func (*nullCapacityAdvertiser) Shutdown() error {
	return nil
}

type k8sNPM struct {
	client *kubernetes.Clientset

	cancel chan struct{}

	nodeLock sync.RWMutex
	nodes    map[string]string
}

// For the moment, ignore the lbgID: all runners belong to all pools.
// In the future, we could use a label to identify these.
func (k *k8sNPM) GetRunners(string) ([]string, error) {
	nodes := []string{}

	k.nodeLock.RLock()
	defer k.nodeLock.RUnlock()

	for _, addr := range k.nodes {
		if addr != "" {
			nodes = append(nodes, addr)
		}
	}

	sort.Strings(nodes)
	return nodes, nil
}

func (k *k8sNPM) AdvertiseCapacity(snapshots *pool_grpc.CapacitySnapshotList) error {
	return nil
}

func (k *k8sNPM) Shutdown() error {
	close(k.cancel)
	return nil
}

// Manage node update and deletion under the hood
func (k *k8sNPM) registerNode(node string, addr string) {
	k.nodeLock.Lock()
	defer k.nodeLock.Unlock()

	if address, ok := k.nodes[node]; ok {
		if address != addr {
			logrus.WithField("node", node).WithField("address", addr).Info("Updating address of registered node")
		} else {
			logrus.WithField("node", node).WithField("address", addr).Debug("Attempt to add extant node at known address")
		}
	}

	k.nodes[node] = addr
}

func (k *k8sNPM) unregisterNode(node string) {
	k.nodeLock.Lock()
	defer k.nodeLock.Unlock()

	if addr, ok := k.nodes[node]; ok {
		logrus.WithField("node", node).WithField("address", addr).Info("Removing registered node")
	} else {
		logrus.WithField("node", node).Warn("Attempt to remove unknown node")
	}

	delete(k.nodes, node)
}

// Kubernetes-specific stuff

func namespace() (string, error) {
	// Use our current namespace
	ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/" + v1.ServiceAccountNamespaceKey)
	if err != nil {
		// If we're not running inside k8s, bail out
		return "", err
	}

	return strings.TrimSpace(string(ns)), nil
}

// Watch for pods under a given namespace matching a particular labelselector.
// As they become ready, register an address derived from their PodIP and the specified port
func (k *k8sNPM) watch(ns string, ls string, port int) {
	pods, err := k.client.CoreV1().Pods(ns).Watch(metav1.ListOptions{LabelSelector: ls})
	if err != nil {
		panic(err.Error())
	}

	logrus.WithField("Namespace", ns).WithField("LabelSelector", ls).Info("Watching for pod changes")

	// This runs forever.
	for {
		select {
		case <-k.cancel:
			logrus.Info("Stopped watching for pod changes")
			pods.Stop()
			return
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
						address = fmt.Sprintf("%s:%d", pod.Status.PodIP, port)
					}
					k.registerNode(pod.SelfLink, address)
				case watch.Deleted:
					logrus.WithField("Pod", pod.SelfLink).WithField("PodIp", pod.Status.PodIP).Debug("Pod removed")
					k.unregisterNode(pod.SelfLink)
				}
			}
		}
	}
}
