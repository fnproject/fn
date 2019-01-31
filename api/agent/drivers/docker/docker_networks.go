package docker

import (
	"math"
	"strings"
	"sync"

	"github.com/fnproject/fn/api/agent/drivers"
)

type DockerNetworks struct {
	// protects networks map
	networksLock sync.Mutex
	networks     map[string]uint64
}

func NewDockerNetworks(conf drivers.Config) *DockerNetworks {
	obj := &DockerNetworks{
		networks: make(map[string]uint64),
	}

	// Record DockerNetworks both in wait-list and in network allocations
	for _, net := range strings.Fields(conf.DockerNetworks) {
		obj.networks[net] = 0
	}

	return obj
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
