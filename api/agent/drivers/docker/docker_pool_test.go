package docker

import (
	"runtime"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
)

func getDefaultCfg() *drivers.Config {
	cfg := &drivers.Config{
		PreForkImage: "busybox",
		PreForkCmd:   "tail -f /dev/null",
	}
	return cfg
}

func TestRunnerDockerPool(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("prefork only supported on Linux")
		return
	}

	cfg := getDefaultCfg()

	// shouldn't spin up a pool since cfg is empty
	drv := NewDocker(*cfg)

	cfg.PreForkPoolSize = 2
	pool := NewDockerPool(*cfg, drv)

	// primitive wait here
	i := 0
	for ; i < 10; i++ {
		stats := pool.Usage()
		if stats.free == 2 {
			break
		}

		<-time.After(time.Duration(500) * time.Millisecond)
	}
	if i == 10 {
		t.Fatalf("pool initialize timeout stats=%+v", pool.Usage())
	}

	id1, err := pool.AllocPoolId()
	if err != nil {
		t.Fatalf("pool AllocPoolId id1 err=%s", err.Error())
	}
	t.Logf("pool AllocPoolId id1=%s", id1)

	id2, err := pool.AllocPoolId()
	if err != nil {
		t.Fatalf("pool AllocPoolId id2 err=%s", err.Error())
	}
	t.Logf("pool AllocPoolId id2=%s", id2)

	id3, err := pool.AllocPoolId()
	if err == nil {
		t.Fatalf("pool AllocPoolId id3 should be err, but got id=%s", id3)
	}
	t.Logf("pool AllocPoolId id3=%s", id3)

	pool.FreePoolId("nonsense")

	id4, err := pool.AllocPoolId()
	if err == nil {
		t.Fatalf("pool AllocPoolId id4 should be err, but got id=%s", id3)
	}
	t.Logf("pool AllocPoolId id4=%s", id4)

	pool.FreePoolId(id1)

	id5, err := pool.AllocPoolId()
	if err != nil {
		t.Fatalf("pool AllocPoolId id5 err=%s", err.Error())
	}
	t.Logf("pool AllocPoolId id5=%s", id5)
	if id5 != id1 {
		t.Fatalf("pool AllocPoolId id5 != id1 (%s != %s)", id5, id1)
	}

	err = pool.Close()
	if err != nil {
		t.Fatalf("pool close err=%s", err.Error())
	}

	err = drv.Close()
	if err != nil {
		t.Fatalf("drv close err=%s", err.Error())
	}

	stats := pool.Usage()
	if stats.free != 0 && stats.inuse != 0 {
		t.Fatalf("pool shutdown timeout stats=%+v", stats)
	}
}

func TestRunnerDockerPoolFaulty(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("prefork only supported on Linux")
		return
	}

	cfg := getDefaultCfg()

	// shouldn't spin up a pool since cfg is empty
	drv := NewDocker(*cfg)

	cfg.PreForkPoolSize = 2
	cfg.PreForkCmd = "sleep 0"

	pool := NewDockerPool(*cfg, drv)

	<-time.After(time.Duration(500) * time.Millisecond)

	// Not much to see if pre-fork has exited, but let's close
	// and wait at least to make sure we don't crash and burn.
	id1, err := pool.AllocPoolId()
	t.Logf("pool AllocPoolId id=%s err=%v", id1, err)
	if id1 != "" {
		pool.FreePoolId(id1)
	}

	<-time.After(time.Duration(500) * time.Millisecond)

	id2, err := pool.AllocPoolId()
	t.Logf("pool AllocPoolId id=%s err=%v", id2, err)
	if id2 != "" {
		pool.FreePoolId(id2)
	}

	err = pool.Close()
	if err != nil {
		t.Fatalf("pool close err=%s", err.Error())
	}

	err = drv.Close()
	if err != nil {
		t.Fatalf("drv close err=%s", err.Error())
	}

	stats := pool.Usage()
	if stats.free != 0 && stats.inuse != 0 {
		t.Fatalf("pool shutdown timeout stats=%+v", stats)
	}
}
