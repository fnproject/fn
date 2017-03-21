package mysql

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"github.com/iron-io/functions/api/datastore/internal/datastoretest"
)

const tmpMysql = "mysql://root:root@tcp(%v:3307)/funcs"

func prepareMysqlTest(logf, fatalf func(string, ...interface{})) (func(), func()) {
	fmt.Println("initializing mysql for test")
	tryRun(logf, "remove old mysql container", exec.Command("docker", "rm", "-f", "iron-mysql-test"))
	mustRun(fatalf, "start mysql container", exec.Command(
		"docker", "run", "--name", "iron-mysql-test", "-p", "3307:3306", "-e", "MYSQL_DATABASE=funcs",
		"-e", "MYSQL_ROOT_PASSWORD=root", "-d", "mysql"))
	maxWait := 16 * time.Second
	wait := 2 * time.Second
	var db *sql.DB
	var err error
	for {
		db, err = sql.Open("mysql", fmt.Sprintf("root:root@tcp(%v:3307)/",
			datastoretest.GetContainerHostIP()))
		if err != nil {
			if wait > maxWait {
				fatalf("failed to connect to mysql after %d seconds", maxWait)
				break
			}
			fmt.Println("failed to connect to mysql:", err)
			fmt.Println("retrying in:", wait)
			time.Sleep(wait)
			continue
		}
		// Ping
		if _, err = db.Exec("SELECT 1"); err != nil {
			fmt.Println("failed to ping database:", err)
			time.Sleep(wait)
			continue
		}
		break
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
			db, err := sql.Open("mysql", fmt.Sprintf("root:root@tcp(%v:3307)/",
				datastoretest.GetContainerHostIP()))
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
			tryRun(logf, "stop mysql container", exec.Command("docker", "rm", "-f", "iron-mysql-test"))
		}
}

func TestDatastore(t *testing.T) {
	_, close := prepareMysqlTest(t.Logf, t.Fatalf)
	defer close()

	u, err := url.Parse(fmt.Sprintf(tmpMysql, datastoretest.GetContainerHostIP()))
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
