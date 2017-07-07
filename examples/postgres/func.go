package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/pkg/errors"

	_ "github.com/lib/pq"
)

var (
	// command to execute, 'SELECT' or 'INSERT'
	command = os.Getenv("HEADER_COMMAND")
	// postgres host:port, e.g. 'postgres:5432'
	server = os.Getenv("HEADER_SERVER")
	// postgres table name
	table = os.Getenv("HEADER_TABLE")
)

func main() {
	req, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to read stdin"))
	}

	db, err := sql.Open("postgres", "postgres://postgres@"+server+"?sslmode=disable")
	if err != nil {
		log.Println("Failed to connect to postgres server")
		log.Fatal(err)
		return
	}

	switch command {
	case "SELECT":
		if resp, err := selectCommand(req, db); err != nil {
			log.Fatal(errors.Wrap(err, "select command failed"))
		} else {
			log.Println(resp)
		}
	case "INSERT":
		if err := insertCommand(req, db); err != nil {
			log.Fatal(errors.Wrap(err, "insert command failed"))
		}
	default:
		log.Fatalf("invalid command: %q", command)
	}
}

func selectCommand(req []byte, db *sql.DB) (string, error) {
	// Parse request JSON
	var params map[string]interface{}
	if err := json.Unmarshal(req, &params); err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}

	// Build query and gather arguments
	var query bytes.Buffer
	var args []interface{}

	query.WriteString("SELECT json_agg(t) FROM (SELECT * FROM ")
	query.WriteString(table)
	query.WriteString(" WHERE")
	first := true
	arg := 1
	for k, v := range params {
		args = append(args, v)

		if !first {
			query.WriteString(" AND")
		}
		query.WriteString(" ")
		query.WriteString(k)
		query.WriteString("=$")
		query.WriteString(strconv.Itoa(arg))
		arg += 1
		first = false
	}
	query.WriteString(") AS t")

	// Execute query
	r := db.QueryRow(query.String(), args...)
	var resp string
	if err := r.Scan(&resp); err != nil {
		return "", errors.Wrap(err, "failed to execute select query")
	}

	return resp, nil
}

func insertCommand(req []byte, db *sql.DB) error {
	q := "INSERT INTO " + table + " SELECT * FROM json_populate_record(null::" + table + ", $1)"
	_, err := db.Exec(q, req)
	if err != nil {
		return errors.Wrap(err, "Failed to execute insert query")
	}
	return nil
}
