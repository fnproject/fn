package tests

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/fnproject/fn/api/server"
	_ "github.com/fnproject/fn/api/server/defaultexts"
)

func stopServer(done chan struct{}, stop func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stop()

	select {
	case <-done:
	case <-ctx.Done():
		log.Panic("Server Cleanup failed, timeout")
	}
}

func startServer() (chan struct{}, func()) {

	log.Print("Starting server")

	ctx, srvCancel := context.WithCancel(context.Background())
	srvDone := make(chan struct{})

	timeString := time.Now().Format("2006_01_02_15_04_05")
	dbURL := os.Getenv(server.EnvDBURL)
	tmpDir := os.TempDir()
	tmpMq := fmt.Sprintf("%s/fn_integration_test_%s_worker_mq.db", tmpDir, timeString)
	tmpDb := fmt.Sprintf("%s/fn_integration_test_%s_fn.db", tmpDir, timeString)
	mqURL := fmt.Sprintf("bolt://%s", tmpMq)
	if dbURL == "" {
		dbURL = fmt.Sprintf("sqlite3://%s", tmpDb)
	}

	srv := server.New(ctx,
		server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)),
		server.WithDBURL(dbURL),
		server.WithMQURL(mqURL),
		server.WithFullAgent(),
	)

	go func() {
		srv.Start(ctx)
		log.Print("Stopped server")
		os.Remove(tmpMq)
		os.Remove(tmpDb)
		close(srvDone)
	}()

	startCtx, startCancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(10)*time.Second))
	defer startCancel()
	for {
		err := checkServer(startCtx)
		if err == nil {
			break
		}
		select {
		case <-time.After(time.Second * 1):
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			log.Panic("Server check failed, timeout")
		}
	}

	return srvDone, srvCancel
}

func TestMain(m *testing.M) {
	done, cancel := startServer()
	// call flag.Parse() here if TestMain uses flags
	result := m.Run()
	stopServer(done, cancel)

	if result == 0 {
		fmt.Fprintln(os.Stdout, "😀  👍  🎗")
	}
	os.Exit(result)
}
