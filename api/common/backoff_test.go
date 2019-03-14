package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func checkRange(bck BackOff, expOk bool, min, max uint64) error {
	delay, ok := bck.NextBackOff()
	if ok != expOk {
		return fmt.Errorf("%v != %v delay=%v", ok, expOk, delay)
	}
	dMax := time.Duration(max) * time.Millisecond
	dMin := time.Duration(min) * time.Millisecond
	if delay > dMax || delay < dMin {
		return fmt.Errorf("%v == %v but %v < %v < %v", ok, expOk, dMin, delay, dMax)
	}
	logrus.Infof("checkRange ok=%v %v < %v < %v", ok, dMin, delay, dMax)
	return nil
}

func TestBackoff1(t *testing.T) {

	cfg := BackOffConfig{
		MaxRetries: 0,
		Interval:   100,
		MaxDelay:   1000,
		MinDelay:   100,
	}

	bck := NewBackOff(cfg)

	for i := 0; i < 32; i++ {
		err := checkRange(bck, false, 0, 0)
		if err != nil {
			t.Fatalf("fail %v", err)
		}
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
		err := checkRange(bck, true, 100, 100)
		if err != nil {
			t.Fatalf("fail %v", err)
		}
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
	var err error

	err = checkRange(bck, true, 1000, 1100)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 1300)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 1700)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 2500)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 4100)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, false, 0, 0)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
}

func TestBackoff4(t *testing.T) {

	cfg := BackOffConfig{
		MaxRetries: 5,
		Interval:   100,
		MaxDelay:   2000,
		MinDelay:   1000,
	}

	bck := NewBackOff(cfg)
	var err error

	err = checkRange(bck, true, 1000, 1100)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 1300)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 1700)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 2000)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 1000, 2000)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, false, 0, 0)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
}

func TestBackoff5(t *testing.T) {

	cfg := BackOffConfig{
		MaxRetries: 5,
		Interval:   1000,
		MaxDelay:   0,
		MinDelay:   0,
	}

	bck := NewBackOff(cfg)
	var err error

	err = checkRange(bck, true, 0, 1000)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 0, 3000)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 0, 7000)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 0, 15000)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, true, 0, 31000)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
	err = checkRange(bck, false, 0, 0)
	if err != nil {
		t.Fatalf("fail %v", err)
	}
}
