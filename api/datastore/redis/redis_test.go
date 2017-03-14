package redis

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/iron-io/functions/api/datastore/internal/datastoretest"
)

const tmpRedis = "redis://%v:6301/"

func prepareRedisTest(logf, fatalf func(string, ...interface{})) (func(), func()) {
	fmt.Println("initializing redis for test")
	tryRun(logf, "remove old redis container", exec.Command("docker", "rm", "-f", "iron-redis-test"))
	mustRun(fatalf, "start redis container", exec.Command("docker", "run", "--name", "iron-redis-test", "-p", "6301:6379", "-d", "redis"))
	timeout := time.After(20 * time.Second)

	for {
		c, err := redis.DialURL(fmt.Sprintf(tmpRedis, datastoretest.GetContainerHostIP()))
		if err == nil {
			_, err = c.Do("PING")
			c.Close()
			if err == nil {
				break
			}
		}
		fmt.Println("failed to PING redis:", err)
		select {
		case <-timeout:
			log.Fatal("timed out waiting for redis")
		case <-time.After(500 * time.Millisecond):
			continue
		}
	}
	fmt.Println("redis for test ready")
	return func() {},
		func() {
			tryRun(logf, "stop redis container", exec.Command("docker", "rm", "-f", "iron-redis-test"))
		}
}

func TestDatastore(t *testing.T) {
	_, close := prepareRedisTest(t.Logf, t.Fatalf)
	defer close()

	u, err := url.Parse(fmt.Sprintf(tmpRedis, datastoretest.GetContainerHostIP()))
	if err != nil {
		t.Fatal("failed to parse url: ", err)
	}
	ds, err := New(u)
	if err != nil {
		t.Fatal("failed to create redis datastore:", err)
	}

	datastoretest.Test(t, ds)
}

func tryRun(logf func(string, ...interface{}), desc string, cmd *exec.Cmd) {
	var b bytes.Buffer
	cmd.Stderr = &b
	if err := cmd.Run(); err != nil {
		logf("failed to %s: %s", desc, b.String())
	}
}

func mustRun(fatalf func(string, ...interface{}), desc string, cmd *exec.Cmd) {
	var b bytes.Buffer
	cmd.Stderr = &b
	if err := cmd.Run(); err != nil {
		fatalf("failed to %s: %s", desc, b.String())
	}
}
