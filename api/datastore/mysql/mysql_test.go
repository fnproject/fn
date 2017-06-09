package mysql

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

const tmpMysql = "mysql://root:root@tcp(%s:%d)/funcs"

var (
	mysqlHost = func() string {
		host := os.Getenv("MYSQL_HOST")
		if host == "" {
			host = "127.0.0.1"
		}
		return host
	}()
	mysqlPort = func() int {
		port := os.Getenv("MYSQL_PORT")
		if port == "" {
			port = "3307"
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			panic(err)
		}
		return p
	}()
)

func prepareMysqlTest(logf, fatalf func(string, ...interface{})) (func(), func()) {
	timeout := time.After(60 * time.Second)
	wait := 2 * time.Second
	var db *sql.DB
	var err error
	var buf bytes.Buffer
	time.Sleep(time.Second * 25)
	for {
		db, err = sql.Open("mysql", fmt.Sprintf("root:root@tcp(%s:%v)/",
			mysqlHost, mysqlPort))
		if err != nil {
			fmt.Fprintln(&buf, "failed to connect to mysql:", err)
			fmt.Fprintln(&buf, "retrying in:", wait)
		} else {
			// Ping
			if _, err = db.Exec("SELECT 1"); err == nil {
				break
			}
			fmt.Fprintln(&buf, "failed to ping database:", err)
		}
		select {
		case <-timeout:
			fmt.Println(buf.String())
			log.Fatal("timed out waiting for mysql")
		case <-time.After(wait):
			continue
		}
	}

	_, err = db.Exec("DROP DATABASE IF EXISTS funcs;")
	if err != nil {
		fmt.Println("failed to drop database:", err)
	}
	_, err = db.Exec("CREATE DATABASE funcs;")
	if err != nil {
		fatalf("failed to create database: %s\n", err)
	}
	_, err = db.Exec(`GRANT ALL PRIVILEGES ON funcs.* TO root@localhost WITH GRANT OPTION;`)
	if err != nil {
		fatalf("failed to grant priviledges to user 'mysql: %s\n", err)
		panic(err)
	}

	fmt.Println("mysql for test ready")
	return func() {
			db, err := sql.Open("mysql", fmt.Sprintf("root:root@tcp(%s:%d)/",
				mysqlHost, mysqlPort))
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
			tryRun(logf, "stop mysql container", exec.Command("docker", "rm", "-vf", "func-mysql-test"))
		}
}

func TestDatastore(t *testing.T) {
	_, close := prepareMysqlTest(t.Logf, t.Fatalf)
	defer close()

	u, err := url.Parse(fmt.Sprintf(tmpMysql, mysqlHost, mysqlPort))
	if err != nil {
		t.Fatalf("failed to parse url: %s\n", err)
	}
	ds, err := New(u)
	if err != nil {
		t.Fatalf("failed to create mysql datastore: %s\n", err)
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
