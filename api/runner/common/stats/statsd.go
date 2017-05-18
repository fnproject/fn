package stats

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
)

type StatsdConfig struct {
	StatsdUdpTarget string `json:"target" mapstructure:"target" envconfig:"STATSD_TARGET"`
	Interval        int64  `json:"interval" envconfig:"STATSD_INTERVAL"`
	Prefix          string `json:"prefix" envconfig:"STATSD_PREFIX"`
}

type keyCreator interface {
	// The return value of Key *MUST* never have a '.' at the end.
	Key(stat string) string
}

type theStatsdReporter struct {
	keyCreator
	client statsd.Statter
}

type prefixKeyCreator struct {
	parent   keyCreator
	prefixes []string
}

func (pkc *prefixKeyCreator) Key(stat string) string {
	prefix := strings.Join(pkc.prefixes, ".")

	if pkc.parent != nil {
		prefix = pkc.parent.Key(prefix)
	}

	if stat == "" {
		return prefix
	}

	if prefix == "" {
		return stat
	}

	return prefix + "." + stat
}

func whoami() string {
	a, _ := net.InterfaceAddrs()
	for i := range a {
		// is a textual representation of an IPv4 address
		z, _, err := net.ParseCIDR(a[i].String())
		if a[i].Network() == "ip+net" && err == nil && z.To4() != nil {
			if !bytes.Equal(z, net.ParseIP("127.0.0.1")) {
				return strings.Replace(fmt.Sprintf("%v", z), ".", "_", -1)
			}
		}
	}
	return "127_0_0_1" // shrug
}

// The config.Prefix is sent before each message and can be used to set API
// keys. The prefix is used as the key prefix.
// If config is nil, creates a noop reporter.
//
//	st, e := NewStatsd(config, "servicename")
//	st.Inc("eventname", 1) -> Actually records to key servicename.eventname.
func NewStatsd(config *StatsdConfig) (*theStatsdReporter, error) {
	var client statsd.Statter
	var err error
	if config != nil {
		// 512 for now since we are sending to hostedgraphite over the internet.
		config.Prefix += "." + whoami()
		client, err = statsd.NewBufferedClient(config.StatsdUdpTarget, config.Prefix, time.Duration(config.Interval)*time.Second, 512)
	} else {
		client, err = statsd.NewNoopClient()
	}
	if err != nil {
		return nil, err
	}

	return &theStatsdReporter{keyCreator: &prefixKeyCreator{}, client: client}, nil
}

func (sr *theStatsdReporter) Inc(component, stat string, value int64, rate float32) {
	sr.client.Inc(sr.keyCreator.Key(component+"."+stat), value, rate)
}

func (sr *theStatsdReporter) Measure(component, stat string, delta int64, rate float32) {
	sr.client.Timing(sr.keyCreator.Key(component+"."+stat), delta, rate)
}

func (sr *theStatsdReporter) Time(component, stat string, delta time.Duration, rate float32) {
	sr.client.TimingDuration(sr.keyCreator.Key(component+"."+stat), delta, rate)
}

func (sr *theStatsdReporter) Gauge(component, stat string, value int64, rate float32) {
	sr.client.Gauge(sr.keyCreator.Key(component+"."+stat), value, rate)
}

func (sr *theStatsdReporter) NewTimer(component string, stat string, rate float32) *Timer {
	return newTimer(sr, component, stat, rate)
}

// We need some kind of all-or-nothing sampler where multiple stats can be
// given the same rate and they are either all logged on that run or none of
// them are. The statsd library we use ends up doing its own rate calculation
// which is going to impede doing something like this.
