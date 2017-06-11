package statsdtest

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
)

func TestRecordingSenderIsSender(t *testing.T) {
	// This ensures that if the Sender interface changes in the future we'll get
	// compile time failures should the RecordingSender not be updated to meet
	// the new definition. This keeps changes from inadvertently breaking tests
	// of folks that use go-statsd-client.
	var _ statsd.Sender = NewRecordingSender()
}

func TestRecordingSender(t *testing.T) {
	start := time.Now()
	rs := new(RecordingSender)
	statter, err := statsd.NewClientWithSender(rs, "test")
	if err != nil {
		t.Errorf("failed to construct client")
		return
	}

	statter.Inc("stat", 4444, 1.0)
	statter.Dec("stat", 5555, 1.0)
	statter.Set("set-stat", "some string", 1.0)

	d := time.Since(start)
	statter.TimingDuration("timing", d, 1.0)

	sent := rs.GetSent()
	if len(sent) != 4 {
		// just dive out because everything else relies on ordering
		t.Fatalf("Did not capture all stats sent; got: %s", sent)
	}

	ms := float64(d) / float64(time.Millisecond)
	// somewhat fragile in that it assums float rendering within client *shrug*
	msStr := string(strconv.AppendFloat([]byte(""), ms, 'f', -1, 64))

	expected := Stats{
		{[]byte("test.stat:4444|c"), "test.stat", "4444", "c", "", true},
		{[]byte("test.stat:-5555|c"), "test.stat", "-5555", "c", "", true},
		{[]byte("test.set-stat:some string|s"), "test.set-stat", "some string", "s", "", true},
		{[]byte(fmt.Sprintf("test.timing:%s|ms", msStr)), "test.timing", msStr, "ms", "", true},
	}

	if !reflect.DeepEqual(sent, expected) {
		t.Errorf("got: %s, want: %s", sent, expected)
	}
}
