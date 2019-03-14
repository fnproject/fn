package common

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func checkRange(t *testing.T, bck BackOff, expOk bool, min, max uint64) {
	delay, ok := bck.NextBackOff()
	if ok != expOk {
		t.Fatalf("%v != %v delay=%v", ok, expOk, delay)
	}
	dMax := time.Duration(max) * time.Millisecond
	dMin := time.Duration(min) * time.Millisecond
	if delay > dMax || delay < dMin {
		t.Fatalf("%v == %v but %v < %v < %v", ok, expOk, dMin, delay, dMax)
	}
	logrus.Infof("checkRange ok=%v %v < %v < %v", ok, dMin, delay, dMax)
}

func TestBackoff1(t *testing.T) {

	cfg := BackOffConfig{
		Interval: 100,
		MaxDelay: 1000,
		MinDelay: 100,
	}

	bck := NewBackOff(cfg)

	for i := 0; i < 32; i++ {
		checkRange(t, bck, false, 0, 0)
	}
}

func TestBackoff2(t *testing.T) {

	cfg := BackOffConfig{
		MaxRetries: RetryForever,
		Interval:   100,
		MaxDelay:   100,
		MinDelay:   100,
	}

	bck := NewBackOff(cfg)

	for i := 0; i < 32; i++ {
		checkRange(t, bck, true, 100, 100)
	}
}

func TestBackoff3(t *testing.T) {

	cfg := BackOffConfig{
		MaxRetries: 5,
		Interval:   100,
		MaxDelay:   10000,
		MinDelay:   1000,
	}

	bck := NewBackOff(cfg)

	checkRange(t, bck, true, 1000, 1100)
	checkRange(t, bck, true, 1000, 1300)
	checkRange(t, bck, true, 1000, 1700)
	checkRange(t, bck, true, 1000, 2500)
	checkRange(t, bck, true, 1000, 4100)
	checkRange(t, bck, false, 0, 0)
}

func TestBackoff4(t *testing.T) {

	cfg := BackOffConfig{
		MaxRetries: 5,
		Interval:   100,
		MaxDelay:   2000,
		MinDelay:   1000,
	}

	bck := NewBackOff(cfg)

	checkRange(t, bck, true, 1000, 1100)
	checkRange(t, bck, true, 1000, 1300)
	checkRange(t, bck, true, 1000, 1700)
	checkRange(t, bck, true, 1000, 2000)
	checkRange(t, bck, true, 1000, 2000)
	checkRange(t, bck, false, 0, 0)
}

func TestBackoff5(t *testing.T) {

	cfg := BackOffConfig{
		MaxRetries: 5,
		Interval:   1000,
	}

	bck := NewBackOff(cfg)

	checkRange(t, bck, true, 0, 1000)
	checkRange(t, bck, true, 0, 3000)
	checkRange(t, bck, true, 0, 7000)
	checkRange(t, bck, true, 0, 15000)
	checkRange(t, bck, true, 0, 31000)
	checkRange(t, bck, false, 0, 0)
}
