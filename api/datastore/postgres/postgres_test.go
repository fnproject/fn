package postgres

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"gitlab-odx.oracle.com/odx/functions/api/datastore/internal/datastoretest"
)

const tmpPostgres = "postgres://postgres@%s:%d/funcs?sslmode=disable"

var (
	postgresHost = func() string {
		host := os.Getenv("POSTGRES_HOST")
		if host == "" {
			host = "127.0.0.1"
		}
		return host
	}()
	postgresPort = func() int {
		port := os.Getenv("POSTGRES_PORT")
		if port == "" {
			port = "15432"
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			panic(err)
		}
		return p
	}()
)

func preparePostgresTest(logf, fatalf func(string, ...interface{})) (func(), func()) {
	timeout := time.After(20 * time.Second)
	wait := 500 * time.Millisecond

	for {
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://postgres@%s:%d?sslmode=disable",
			postgresHost, postgresPort))
		if err != nil {
			fmt.Println("failed to connect to postgres:", err)
			fmt.Println("retrying in:", wait)
		} else {
			_, err = db.Exec(`CREATE DATABASE funcs;`)
			if err != nil {
				fmt.Println("failed to create database:", err)
				fmt.Println("retrying in:", wait)

			} else {
				_, err = db.Exec(`GRANT ALL PRIVILEGES ON DATABASE funcs TO postgres;`)
				if err == nil {
					break
				}
				fmt.Println("failed to grant privileges:", err)
				fmt.Println("retrying in:", wait)
			}

		}
		select {
		case <-timeout:
			log.Fatal("timed out waiting for postgres")
		case <-time.After(wait):
			continue
		}
	}
	fmt.Println("postgres for test ready")
	return func() {
			db, err := sql.Open("postgres", fmt.Sprintf(tmpPostgres, postgresHost, postgresPort))
			if err != nil {
				fatalf("failed to connect for truncation: %s\n", err)
			}
			for _, table := range []string{"routes", "apps", "extras"} {
				_, err = db.Exec(`TRUNCATE TABLE ` + table)
				if err != nil {
					fatalf("failed to truncate table %q: %s\n", table, err)
				}
			}
		},
		func() {
			tryRun(logf, "stop postgres container", exec.Command("docker", "rm", "-fv", "func-postgres-test"))
		}
}

func TestDatastore(t *testing.T) {
	_, close := preparePostgresTest(t.Logf, t.Fatalf)
	defer close()

	u, err := url.Parse(fmt.Sprintf(tmpPostgres, postgresHost, postgresPort))
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	ds, err := New(u)
	if err != nil {
		t.Fatalf("failed to create postgres datastore: %v", err)
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
