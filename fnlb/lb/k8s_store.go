package lb

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

var _ DBStore = &k8sStore{}

func NewK8sClient(conf Config) (*kubernetes.Clientset, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(k8sConfig)
}

func NewK8sStore(conf Config) (*k8sStore, error) {
	client, err := NewK8sClient(conf)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	ns := conf.Namespace
	if ns == "" {
		ns, err = namespace()
		if err != nil {
			return nil, err
		}
	}

	_, err = labels.Parse(conf.LabelSelector)
	if err != nil {
		return nil, err
	}

	k := &k8sStore{
		client: client,

		ctx:    ctx,
		cancel: cancel,

		nodes: map[string]string{},
	}

	go k.watch(ns, conf.LabelSelector, conf.TargetPort)
	return k, nil
}

type k8sStore struct {
	client *kubernetes.Clientset

	ctx    context.Context
	cancel context.CancelFunc

	nodeLock sync.RWMutex
	nodes    map[string]string
}

func (*k8sStore) Add(string) error {
	return errors.New("kubernetes driver does not support manual addition of targets")
}

func (*k8sStore) Delete(string) error {
	return errors.New("kubernetes driver does not support manual deletion of targets")
}

func (k *k8sStore) List() ([]string, error) {
	nodes := []string{}

	k.nodeLock.RLock()
	defer k.nodeLock.RUnlock()

	for _, addr := range k.nodes {
		if addr != "" {
			nodes = append(nodes, addr)
		}
	}

	return nodes, nil
}

func (k *k8sStore) Close() error {
	k.cancel()
	return nil
}

// Manage node update and deletion under the hood
func (k *k8sStore) registerNode(node string, addr string) {
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

func (k *k8sStore) unregisterNode(node string) {
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
func (k *k8sStore) watch(ns string, ls string, port int) {
	pods, err := k.client.CoreV1().Pods(ns).Watch(metav1.ListOptions{LabelSelector: ls})
	if err != nil {
		panic(err.Error())
	}

	logrus.WithField("Namespace", ns).WithField("LabelSelector", ls).Info("Watching for pod changes")

	// This runs forever.
	for {
		select {
		case <-k.ctx.Done():
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
