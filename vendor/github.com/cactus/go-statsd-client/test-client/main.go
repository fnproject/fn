// Copyright (c) 2012-2016 Eli Janssen
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	flags "github.com/jessevdk/go-flags"
)

func main() {

	// command line flags
	var opts struct {
		HostPort  string        `long:"host" default:"127.0.0.1:8125" description:"host:port of statsd server"`
		Prefix    string        `long:"prefix" default:"test-client" description:"Statsd prefix"`
		StatType  string        `long:"type" default:"count" description:"stat type to send. Can be one of: timing, count, guage"`
		StatValue int64         `long:"value" default:"1" description:"Value to send"`
		Name      string        `short:"n" long:"name" default:"counter" description:"stat name"`
		Rate      float32       `short:"r" long:"rate" default:"1.0" description:"sample rate"`
		Volume    int           `short:"c" long:"count" default:"1000" description:"Number of stats to send. Volume."`
		Nil       bool          `long:"nil" description:"Use nil client"`
		Buffered  bool          `long:"buffered" description:"Use a buffered client"`
		Duration  time.Duration `short:"d" long:"duration" default:"10s" description:"How long to spread the volume across. For each second of duration, volume/seconds events will be sent."`
	}

	// parse said flags
	_, err := flags.Parse(&opts)
	if err != nil {
		if e, ok := err.(*flags.Error); ok {
			if e.Type == flags.ErrHelp {
				os.Exit(0)
			}
		}
		fmt.Printf("Error: %+v\n", err)
		os.Exit(1)
	}

	if opts.Nil && opts.Buffered {
		fmt.Printf("Specifying both nil and buffered together is invalid\n")
		os.Exit(1)
	}

	if opts.Name == "" || statsd.CheckName(opts.Name) != nil {
		fmt.Printf("Stat name contains invalid characters\n")
		os.Exit(1)
	}

	if statsd.CheckName(opts.Prefix) != nil {
		fmt.Printf("Stat prefix contains invalid characters\n")
		os.Exit(1)
	}

	var client statsd.Statter
	if !opts.Nil {
		if !opts.Buffered {
			client, err = statsd.NewClient(opts.HostPort, opts.Prefix)
		} else {
			client, err = statsd.NewBufferedClient(opts.HostPort, opts.Prefix, opts.Duration/time.Duration(4), 0)
		}
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
	}

	var stat func(stat string, value int64, rate float32) error
	switch opts.StatType {
	case "count":
		stat = func(stat string, value int64, rate float32) error {
			return client.Inc(stat, value, rate)
		}
	case "gauge":
		stat = func(stat string, value int64, rate float32) error {
			return client.Gauge(stat, value, rate)
		}
	case "timing":
		stat = func(stat string, value int64, rate float32) error {
			return client.Timing(stat, value, rate)
		}
	default:
		log.Fatal("Unsupported state type")
	}

	pertick := opts.Volume / int(opts.Duration.Seconds()) / 10
	// add some extra time, because the first tick takes a while
	ender := time.After(opts.Duration + 100*time.Millisecond)
	c := time.Tick(time.Second / 10)
	count := 0
	for {
		select {
		case <-c:
			for x := 0; x < pertick; x++ {
				err := stat(opts.Name, opts.StatValue, opts.Rate)
				if err != nil {
					log.Printf("Got Error: %+v\n", err)
					break
				}
				count++
			}
		case <-ender:
			log.Printf("%d events called\n", count)
			os.Exit(0)
			return
		}
	}
}
