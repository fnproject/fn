// +build riemann

package stats

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/amir/raidman"
)

type RiemannClient struct {
	client     *raidman.Client
	attributes map[string]string
}

const (
	StateNormal = "normal"
)

func (rc *RiemannClient) Report([]*Stat) {}

func (rc *RiemannClient) Add(s *Stat) {
	var events []*raidman.Event

	t := time.Now().UnixNano()

	for k, v := range rc.attributes {
		s.Tags[k] = v
	}

	for k, v := range s.Counters {
		events = append(events, &raidman.Event{
			Ttl:        5.0,
			Time:       t,
			State:      StateNormal,
			Service:    s.Name + " " + k,
			Metric:     v,
			Attributes: s.Tags,
		})
	}

	for k, v := range s.Values {
		events = append(events, &raidman.Event{
			Ttl:        5.0,
			Time:       t,
			State:      StateNormal,
			Service:    s.Name + " " + k,
			Metric:     v,
			Attributes: s.Tags,
		})
	}

	rc.report(events)
}

func (rc *RiemannClient) report(events []*raidman.Event) {
	err := rc.client.SendMulti(events)
	if err != nil {
		logrus.WithError(err).Error("error sending to Riemann")
	}
}

func (rc *RiemannClient) heartbeat() {
	events := []*raidman.Event{
		&raidman.Event{
			Ttl:        5.0,
			Time:       time.Now().UnixNano(),
			State:      StateNormal,
			Service:    "heartbeat",
			Metric:     1.0,
			Attributes: rc.attributes,
		},
	}
	rc.report(events)
}

func newRiemann(config Config) *RiemannClient {
	c, err := raidman.Dial("tcp", config.Riemann.RiemannHost)
	if err != nil {
		logrus.WithError(err).Error("error dialing Riemann")
		os.Exit(1)
	}

	client := &RiemannClient{
		client:     c,
		attributes: map[string]string{},
	}

	for k, v := range config.Tags {
		client.attributes[k] = v
	}

	// Send out a heartbeat every second
	go func(rc *RiemannClient) {
		for _ = range time.Tick(1 * time.Second) {
			rc.heartbeat()
		}
	}(client)

	return client
}
