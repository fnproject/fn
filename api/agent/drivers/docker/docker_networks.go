package docker

import (
	"context"
	"math"
	"strings"
	"sync"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type DockerNetworks struct {
	// protects networks map
	networksLock sync.Mutex
	networks     map[string]uint64

	// read-only map for unlocked reads for WaitDockerNetworks
	waitList      map[string]struct{}
	waitListReady chan struct{}
}

func NewDockerNetworks(conf drivers.Config) *DockerNetworks {
	obj := &DockerNetworks{
		networks:      make(map[string]uint64),
		waitList:      make(map[string]struct{}),
		waitListReady: make(chan struct{}),
	}

	// Record DockerNetworks both in wait-list and in network allocations
	for _, net := range strings.Fields(conf.DockerNetworks) {
		obj.networks[net] = 0
		obj.waitList[net] = struct{}{}
	}

	// IMPORTANT: if PreForkPool is enabled with additional networks, add those in wait-list
	if conf.PreForkPoolSize != 0 {
		for _, net := range strings.Fields(conf.PreForkNetworks) {
			obj.waitList[net] = struct{}{}
		}
	}

	return obj
}

func (n *DockerNetworks) isDockerNetworkReady() bool {
	if len(n.waitList) > 0 {
		select {
		case <-n.waitListReady:
		default:
			return false
		}
	}
	return true
}

func pollNetworkEvents(ctx context.Context, driver *DockerDriver, collector chan string) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "pollEvents"})

	listener, err := driver.docker.AddEventListener(ctx)
	if err != nil {
		log.WithError(err).Error("AddEventListener failed, will retry...")
		return
	}

	defer driver.docker.RemoveEventListener(ctx, listener)

	for ctx.Err() == nil {
		select {
		case ev := <-listener:
			if ev == nil {
				log.WithError(err).Error("event listener closed, will retry...")
				return
			}
			if ev.Action == "create" && ev.Type == "network" {
				if name, ok := ev.Actor.Attributes["name"]; ok {
					select {
					case collector <- name:
					case <-ctx.Done():
					}
				}
			}
		case <-ctx.Done():
		}
	}
}

func pollNetworkList(ctx context.Context, driver *DockerDriver, collector chan string) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "pollNetworkList"})

	networks, err := driver.docker.ListNetworks(ctx)
	if err != nil {
		log.WithError(err).Error("ListNetworks failed, will retry...")
		return
	}

	for _, item := range networks {
		select {
		case collector <- item.Name:
		case <-ctx.Done():
			return
		}
	}
}

func (n *DockerNetworks) WaitDockerNetworks(ctx context.Context, driver *DockerDriver) {
	if len(n.waitList) == 0 {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// for logging
	names := make([]string, 0, len(n.waitList))
	for key, _ := range n.waitList {
		names = append(names, key)
	}

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"stack": "waitDockerNetworks", "networks": names})
	log.Info("Checking status of docker networks")

	collector := make(chan string)

	// stream network events, if fails, reconnect every 2 secs
	go func() {
		limiter := rate.NewLimiter(0.5, 1)
		for limiter.Wait(ctx) == nil {
			pollNetworkEvents(ctx, driver, collector)
		}
	}()
	// poll network list, every 2 secs
	go func() {
		limiter := rate.NewLimiter(0.5, 1)
		for limiter.Wait(ctx) == nil {
			pollNetworkList(ctx, driver, collector)
		}
	}()

	// if we find the networks either in docker-events or docker-list-networks, we proceed
	checker := make(map[string]struct{})
	for {
		select {
		case network := <-collector:
			if _, ok := n.waitList[network]; ok {
				checker[network] = struct{}{}
			}
		case <-ctx.Done():
			return
		}

		if len(checker) == len(n.waitList) {
			close(n.waitListReady)
			log.Info("All docker networks are ready")
			return
		}
	}
}

// pick least used network
func (n *DockerNetworks) AllocNetwork() string {
	if len(n.networks) == 0 {
		return ""
	}

	var id string
	min := uint64(math.MaxUint64)

	n.networksLock.Lock()
	for key, val := range n.networks {
		if val < min {
			id = key
			min = val
		}
	}
	n.networks[id]++
	n.networksLock.Unlock()

	return id
}

// unregister network
func (n *DockerNetworks) FreeNetwork(id string) {
	n.networksLock.Lock()
	if count, ok := n.networks[id]; ok {
		n.networks[id] = count - 1
	}
	n.networksLock.Unlock()
}
