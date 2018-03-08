/*
	Interface between the Node Pool Manager and the Control Plane
*/

package cp

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"
)

type Runner struct {
	Id      string
	Address string
	// Other: certs etc here as managed and installed by CP
	Capacity int64
}

const CAPACITY_PER_RUNNER = 4096

type ControlPlane interface {
	GetLBGRunners(lgbId string) ([]*Runner, error)
	ProvisionRunners(lgbId string, n int) (int, error)
	RemoveRunner(lbgId string, id string) error
}

type controlPlane struct {
	mx sync.RWMutex

	runners map[string][]*Runner

	_fakeRunners []string
}

const REQUEST_DURATION = 5 * time.Second

func NewControlPlane(fakeRunners []string) ControlPlane {
	cp := &controlPlane{
		runners: make(map[string][]*Runner),

		_fakeRunners: fakeRunners,
	}
	return cp
}

func (cp *controlPlane) GetLBGRunners(lbgId string) ([]*Runner, error) {
	cp.mx.RLock()
	defer cp.mx.RUnlock()

	runners := make([]*Runner, 0)
	if hosts, ok := cp.runners[lbgId]; ok {
		for _, host := range hosts {
			runners = append(runners, host) // In this CP implementation, a Runner is an immutable thing, so passing the pointer is fine
		}
	}

	return runners, nil
}

func (cp *controlPlane) ProvisionRunners(lbgId string, n int) (int, error) {
	// Simulate some small amount of time for the CP to service this request
	go func() {
		time.Sleep(REQUEST_DURATION)
		cp.mx.Lock()
		defer cp.mx.Unlock()

		runners, ok := cp.runners[lbgId]
		if !ok {
			runners = make([]*Runner, 0)
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
func (cp *controlPlane) makeRunners(lbg string) []*Runner {

	var runners []*Runner
	for _, fakeRunner := range cp._fakeRunners {

		b := make([]byte, 16)
		_, err := rand.Read(b)
		if err != nil {
			log.Panic("Error constructing UUID for runner: ", err)
		}

		uuid := fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

		runners = append(runners, &Runner{
			Id:       uuid,
			Address:  fakeRunner,
			Capacity: CAPACITY_PER_RUNNER,
		})
	}
	return runners
}

// Ditch a runner from the pool.
// We do this immediately - no point modelling a wait here
// note: if this actually took time, we'd want to detect this properly so as to not confuse the NPM
func (cp *controlPlane) RemoveRunner(lbgId string, id string) error {
	cp.mx.Lock()
	defer cp.mx.Unlock()

	if runners, ok := cp.runners[lbgId]; ok {
		newRunners := make([]*Runner, 0)
		for _, host := range runners {
			if host.Id != id {
				newRunners = append(newRunners, host)
			}
		}
		cp.runners[lbgId] = newRunners
	}
	return nil
}
