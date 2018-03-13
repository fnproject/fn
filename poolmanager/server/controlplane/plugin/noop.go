/**
 * Dummy implementation for the controlplane that just adds delays
 */
package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fnproject/fn/poolmanager/server/controlplane"
)

const (
	EnvFixedRunners = "FN_RUNNER_ADDRESSES"
)

type noopControlPlane struct {
	mx           sync.RWMutex
	runners      map[string][]*controlplane.Runner
	_fakeRunners []string
}

const REQUEST_DURATION = 5 * time.Second

func init() {
	ControlPlane = noopControlPlane{
		runners:      make(map[string][]*controlplane.Runner),
		_fakeRunners: strings.Split(getEnv(EnvFixedRunners), ","),
	}
}

func main() {
}

func (cp *noopControlPlane) GetLBGRunners(lbgId string) ([]*controlplane.Runner, error) {
	cp.mx.RLock()
	defer cp.mx.RUnlock()

	runners := make([]*controlplane.Runner, 0)
	if hosts, ok := cp.runners[lbgId]; ok {
		for _, host := range hosts {
			runners = append(runners, host) // In this CP implementation, a Runner is an immutable thing, so passing the pointer is fine
		}
	}

	return runners, nil
}

func (cp *noopControlPlane) ProvisionRunners(lbgId string, n int) (int, error) {
	// Simulate some small amount of time for the CP to service this request
	go func() {
		time.Sleep(REQUEST_DURATION)
		cp.mx.Lock()
		defer cp.mx.Unlock()

		runners, ok := cp.runners[lbgId]
		if !ok {
			runners = make([]*controlplane.Runner, 0)
		}
		for i := 0; i < n; i++ {
			runners = append(runners, cp.makeRunners(lbgId)...)
		}
		cp.runners[lbgId] = runners
	}()
	// How many did we actually ask for?
	return n, nil
}

// Make runner(s)
func (cp *noopControlPlane) makeRunners(lbg string) []*controlplane.Runner {

	var runners []*controlplane.Runner
	for _, fakeRunner := range cp._fakeRunners {

		b := make([]byte, 16)
		_, err := rand.Read(b)
		if err != nil {
			log.Panic("Error constructing UUID for runner: ", err)
		}

		uuid := fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

		runners = append(runners, &controlplane.Runner{
			Id:       uuid,
			Address:  fakeRunner,
			Capacity: controlplane.CapacityPerRunner,
		})
	}
	return runners
}

// Ditch a runner from the pool.
// We do this immediately - no point modelling a wait here
// note: if this actually took time, we'd want to detect this properly so as to not confuse the NPM
func (cp *noopControlPlane) RemoveRunner(lbgId string, id string) error {
	cp.mx.Lock()
	defer cp.mx.Unlock()

	if runners, ok := cp.runners[lbgId]; ok {
		newRunners := make([]*controlplane.Runner, 0)
		for _, host := range runners {
			if host.Id != id {
				newRunners = append(newRunners, host)
			}
		}
		cp.runners[lbgId] = newRunners
	}
	return nil
}

func getEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Panicf("Missing config key: %v", key)
	}
	return value
}

var ControlPlane noopControlPlane
