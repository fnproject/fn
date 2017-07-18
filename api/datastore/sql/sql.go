package sql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
	"gitlab-odx.oracle.com/odx/functions/api/datastore/internal/datastoreutil"
	"gitlab-odx.oracle.com/odx/functions/api/models"
)

// this aims to be an ANSI-SQL compliant package that uses only question
// mark syntax for var placement, leaning on sqlx to make compatible all
// queries to the actual underlying datastore.
//
// currently tested and working are postgres, mysql and sqlite3.

var tables = [...]string{`CREATE TABLE IF NOT EXISTS routes (
	app_name varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	memory int NOT NULL,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	type varchar(16) NOT NULL,
	headers text NOT NULL,
	config text NOT NULL,
	PRIMARY KEY (app_name, path)
);`,

	`CREATE TABLE IF NOT EXISTS apps (
	name varchar(256) NOT NULL PRIMARY KEY,
	config text NOT NULL
);`,

	`CREATE TABLE IF NOT EXISTS calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_name varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	PRIMARY KEY (id)
);`,

	`CREATE TABLE IF NOT EXISTS logs (
	id varchar(256) NOT NULL PRIMARY KEY,
	log text NOT NULL
);`,
}

const (
	routeSelector = `SELECT app_name, path, image, format, memory, type, timeout, idle_timeout, headers, config FROM routes`
	callSelector  = `SELECT id, created_at, started_at, completed_at, status, app_name, path FROM calls`
)

type sqlStore struct {
	db *sqlx.DB

	// TODO we should prepare all of the statements, rebind them
	// and store them all here.
}

// New will open the db specified by url, create any tables necessary
// and return a models.Datastore safe for concurrent usage.
func New(url *url.URL) (models.Datastore, error) {
	driver := url.Scheme

	// driver must be one of these for sqlx to work, double check:
	switch driver {
	case "postgres", "pgx", "mysql", "sqlite3", "oci8", "ora", "goracle":
	default:
		return nil, errors.New("invalid db driver, refer to the code")
	}

	if driver == "sqlite3" {
		// make all the dirs so we can make the file..
		dir := filepath.Dir(url.Path)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}

	uri := url.String()
	if driver != "postgres" {
		// postgres seems to need this as a prefix in lib/pq, everyone else wants it stripped of scheme
		uri = strings.TrimPrefix(url.String(), url.Scheme+"://")
	}

	sqldb, err := sql.Open(driver, uri)
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't open db")
		return nil, err
	}

	db := sqlx.NewDb(sqldb, driver)
	// force a connection and test that it worked
	err = db.Ping()
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't ping db")
		return nil, err
	}

	maxIdleConns := 256 // TODO we need to strip this out of the URL probably
	db.SetMaxIdleConns(maxIdleConns)
	logrus.WithFields(logrus.Fields{"max_idle_connections": maxIdleConns, "datastore": driver}).Info("datastore dialed")

	for _, v := range tables {
		_, err = db.Exec(v)
		if err != nil {
			return nil, err
		}
	}

	sqlDatastore := &sqlStore{db: db}
	return datastoreutil.NewValidator(sqlDatastore), nil
}

func (ds *sqlStore) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	var cbyte []byte
	var err error
	if app.Config != nil {
		cbyte, err = json.Marshal(app.Config)
		if err != nil {
			return nil, err
		}
	}

	query := ds.db.Rebind("INSERT INTO apps (name, config) VALUES (?, ?);")
	_, err = ds.db.Exec(query, app.Name, string(cbyte))
	if err != nil {
		switch err := err.(type) {
		case *mysql.MySQLError:
			if err.Number == 1062 {
				return nil, models.ErrAppsAlreadyExists
			}
		case *pq.Error:
			if err.Code == "23505" {
				return nil, models.ErrAppsAlreadyExists
			}
		case sqlite3.Error:
			if err.ExtendedCode == sqlite3.ErrConstraintUnique || err.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
				return nil, models.ErrAppsAlreadyExists
			}
		}
		return nil, err
	}

	return app, nil
}

func (ds *sqlStore) UpdateApp(ctx context.Context, newapp *models.App) (*models.App, error) {
	app := &models.App{Name: newapp.Name}
	err := ds.Tx(func(tx *sqlx.Tx) error {
		query := tx.Rebind(`SELECT config FROM apps WHERE name=?`)
		row := tx.QueryRow(query, app.Name)

		var config string
		if err := row.Scan(&config); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			}
			return err
		}

		if config != "" {
			err := json.Unmarshal([]byte(config), &app.Config)
			if err != nil {
				return err
			}
		}

		app.UpdateConfig(newapp.Config)

		cbyte, err := json.Marshal(app.Config)
		if err != nil {
			return err
		}

		query = tx.Rebind(`UPDATE apps SET config=? WHERE name=?`)
		res, err := tx.Exec(query, string(cbyte), app.Name)
		if err != nil {
			return err
		}

		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return models.ErrAppsNotFound
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return app, nil
}

func (ds *sqlStore) RemoveApp(ctx context.Context, appName string) error {
	query := ds.db.Rebind(`DELETE FROM apps WHERE name = ?`)
	_, err := ds.db.Exec(query, appName)
	return err
}

func (ds *sqlStore) GetApp(ctx context.Context, name string) (*models.App, error) {
	query := ds.db.Rebind(`SELECT name, config FROM apps WHERE name=?`)
	row := ds.db.QueryRow(query, name)

	var resName, config string
	err := row.Scan(&resName, &config)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrAppsNotFound
		}
		return nil, err
	}

	res := &models.App{
		Name: resName,
	}

	if len(config) > 0 {
		err := json.Unmarshal([]byte(config), &res.Config)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

// GetApps retrieves an array of apps according to a specific filter.
func (ds *sqlStore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}
	query, args := buildFilterAppQuery(filter)
	query = ds.db.Rebind(fmt.Sprintf("SELECT DISTINCT name, config FROM apps %s", query))
	rows, err := ds.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var app models.App
		err := scanApp(rows, &app)

		if err != nil {
			if err == sql.ErrNoRows {
				return res, nil
			}
			return res, err
		}
		res = append(res, &app)
	}

	if err := rows.Err(); err != nil {
		return res, err
	}
	return res, nil
}

func (ds *sqlStore) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	hbyte, err := json.Marshal(route.Headers)
	if err != nil {
		return nil, err
	}

	cbyte, err := json.Marshal(route.Config)
	if err != nil {
		return nil, err
	}

	err = ds.Tx(func(tx *sqlx.Tx) error {
		query := tx.Rebind(`SELECT 1 FROM apps WHERE name=?`)
		r := tx.QueryRow(query, route.AppName)
		if err := r.Scan(new(int)); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			}
		}
		query = tx.Rebind(`SELECT 1 FROM routes WHERE app_name=? AND path=?`)
		same, err := tx.Query(query, route.AppName, route.Path)
		if err != nil {
			return err
		}
		defer same.Close()
		if same.Next() {
			return models.ErrRoutesAlreadyExists
		}

		query = tx.Rebind(`INSERT INTO routes (
			app_name,
			path,
			image,
			format,
			memory,
			type,
			timeout,
			idle_timeout,
			headers,
			config
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)

		_, err = tx.Exec(query,
			route.AppName,
			route.Path,
			route.Image,
			route.Format,
			route.Memory,
			route.Type,
			route.Timeout,
			route.IdleTimeout,
			string(hbyte),
			string(cbyte),
		)

		return err
	})

	return route, err
}

func (ds *sqlStore) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	var route models.Route
	err := ds.Tx(func(tx *sqlx.Tx) error {
		query := tx.Rebind(fmt.Sprintf("%s WHERE app_name=? AND path=?", routeSelector))
		row := tx.QueryRow(query, newroute.AppName, newroute.Path)
		if err := scanRoute(row, &route); err == sql.ErrNoRows {
			return models.ErrRoutesNotFound
		} else if err != nil {
			return err
		}

		route.Update(newroute)

		hbyte, err := json.Marshal(route.Headers)
		if err != nil {
			return err
		}

		cbyte, err := json.Marshal(route.Config)
		if err != nil {
			return err
		}

		query = tx.Rebind(`UPDATE routes SET
			image = ?,
			format = ?,
			memory = ?,
			type = ?,
			timeout = ?,
			idle_timeout = ?,
			headers = ?,
			config = ?
		WHERE app_name=? AND path=?;`)

		res, err := tx.Exec(query,
			route.Image,
			route.Format,
			route.Memory,
			route.Type,
			route.Timeout,
			route.IdleTimeout,
			string(hbyte),
			string(cbyte),
			route.AppName,
			route.Path,
		)

		if err != nil {
			return err
		}

		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			// inside of the transaction, we are querying for the row, so we know that it exists
			return nil
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (ds *sqlStore) RemoveRoute(ctx context.Context, appName, routePath string) error {
	query := ds.db.Rebind(`DELETE FROM routes WHERE path = ? AND app_name = ?`)
	res, err := ds.db.Exec(query, routePath, appName)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return models.ErrRoutesNotFound
	}

	return nil
}

func (ds *sqlStore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	rSelectCondition := "%s WHERE app_name=? AND path=?"
	query := ds.db.Rebind(fmt.Sprintf(rSelectCondition, routeSelector))
	row := ds.db.QueryRow(query, appName, routePath)

	var route models.Route
	err := scanRoute(row, &route)
	if err == sql.ErrNoRows {
		return nil, models.ErrRoutesNotFound
	} else if err != nil {
		return nil, err
	}
	return &route, nil
}

// GetRoutes retrieves an array of routes according to a specific filter.
func (ds *sqlStore) GetRoutes(ctx context.Context, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	query, args := buildFilterRouteQuery(filter)
	query = fmt.Sprintf("%s %s", routeSelector, query)
	query = ds.db.Rebind(query)
	rows, err := ds.db.Query(query, args...)
	// todo: check for no rows so we don't respond with a sql 500 err
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var route models.Route
		err := scanRoute(rows, &route)
		if err != nil {
			continue
		}
		res = append(res, &route)

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

/*
GetRoutesByApp retrieves a route with a specific app name.
*/
func (ds *sqlStore) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	var filterQuery string
	var args []interface{}
	if filter == nil {
		filterQuery = "WHERE app_name = ?"
		args = []interface{}{appName}
	} else {
		filter.AppName = appName
		filterQuery, args = buildFilterRouteQuery(filter)
	}

	query := fmt.Sprintf("%s %s", routeSelector, filterQuery)
	query = ds.db.Rebind(query)
	rows, err := ds.db.Query(query, args...)
	// todo: check for no rows so we don't respond with a sql 500 err
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var route models.Route
		err := scanRoute(rows, &route)
		if err != nil {
			continue
		}
		res = append(res, &route)

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

func (ds *sqlStore) Tx(f func(*sqlx.Tx) error) error {
	tx, err := ds.db.Beginx()
	if err != nil {
		return err
	}
	err = f(tx)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (ds *sqlStore) InsertTask(ctx context.Context, task *models.Task) error {
	query := ds.db.Rebind(`INSERT INTO calls (
		id,
		created_at,
		started_at,
		completed_at,
		status,
		app_name,
		path
	)
	VALUES (?, ?, ?, ?, ?, ?, ?);`)

	_, err := ds.db.Exec(query, task.ID, task.CreatedAt.String(),
		task.StartedAt.String(), task.CompletedAt.String(),
		task.Status, task.AppName, task.Path)
	if err != nil {
		return err
	}

	return nil
}

func (ds *sqlStore) GetTask(ctx context.Context, callID string) (*models.FnCall, error) {
	query := fmt.Sprintf(`%s WHERE id=?`, callSelector)
	query = ds.db.Rebind(query)
	row := ds.db.QueryRow(query, callID)

	var call models.FnCall
	err := scanCall(row, &call)
	if err != nil {
		return nil, err
	}
	return &call, nil
}

func (ds *sqlStore) GetTasks(ctx context.Context, filter *models.CallFilter) (models.FnCalls, error) {
	res := models.FnCalls{}
	query, args := buildFilterCallQuery(filter)
	query = fmt.Sprintf("%s %s", callSelector, query)
	query = ds.db.Rebind(query)
	rows, err := ds.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var call models.FnCall
		err := scanCall(rows, &call)
		if err != nil {
			continue
		}
		res = append(res, &call)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *sqlStore) InsertLog(ctx context.Context, callID, callLog string) error {
	query := ds.db.Rebind(`INSERT INTO logs (id, log) VALUES (?, ?);`)
	_, err := ds.db.Exec(query, callID, callLog)
	return err
}

func (ds *sqlStore) GetLog(ctx context.Context, callID string) (*models.FnCallLog, error) {
	query := ds.db.Rebind(`SELECT log FROM logs WHERE id=?`)
	row := ds.db.QueryRow(query, callID)

	var log string
	err := row.Scan(&log)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrCallLogNotFound
		}
		return nil, err
	}

	return &models.FnCallLog{
		CallID: callID,
		Log:    log,
	}, nil
}

func (ds *sqlStore) DeleteLog(ctx context.Context, callID string) error {
	query := ds.db.Rebind(`DELETE FROM logs WHERE id=?`)
	_, err := ds.db.Exec(query, callID)
	return err
}

// TODO scrap for sqlx scanx ?? some things aren't perfect (e.g. config is a json string)
type RowScanner interface {
	Scan(dest ...interface{}) error
}

func ScanLog(scanner RowScanner, log *models.FnCallLog) error {
	return scanner.Scan(
		&log.CallID,
		&log.Log,
	)
}

func scanRoute(scanner RowScanner, route *models.Route) error {
	var headerStr string
	var configStr string

	err := scanner.Scan(
		&route.AppName,
		&route.Path,
		&route.Image,
		&route.Format,
		&route.Memory,
		&route.Type,
		&route.Timeout,
		&route.IdleTimeout,
		&headerStr,
		&configStr,
	)
	if err != nil {
		return err
	}

	if len(headerStr) > 0 {
		err = json.Unmarshal([]byte(headerStr), &route.Headers)
		if err != nil {
			return err
		}
	}

	if len(configStr) > 0 {
		err = json.Unmarshal([]byte(configStr), &route.Config)
		if err != nil {
			return err
		}
	}

	return nil
}

func scanApp(scanner RowScanner, app *models.App) error {
	var configStr string

	err := scanner.Scan(
		&app.Name,
		&configStr,
	)
	if err != nil {
		return err
	}
	if len(configStr) > 0 {
		err = json.Unmarshal([]byte(configStr), &app.Config)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildFilterRouteQuery(filter *models.RouteFilter) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}
	var b bytes.Buffer
	var args []interface{}

	where := func(colOp, val string) {
		if val != "" {
			args = append(args, val)
			if len(args) == 1 {
				fmt.Fprintf(&b, `WHERE %s?`, colOp)
			} else {
				fmt.Fprintf(&b, ` AND %s?`, colOp)
			}
		}
	}

	where("path=", filter.Path)
	where("app_name=", filter.AppName)
	where("image=", filter.Image)

	return b.String(), args
}

func buildFilterAppQuery(filter *models.AppFilter) (string, []interface{}) {
	if filter == nil || filter.Name == "" {
		return "", nil
	}

	return "WHERE name LIKE ?", []interface{}{filter.Name}
}

func buildFilterCallQuery(filter *models.CallFilter) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}
	var b bytes.Buffer
	var args []interface{}

	where := func(colOp, val string) {
		if val != "" {
			args = append(args, val)
			if len(args) == 1 {
				fmt.Fprintf(&b, `WHERE %s?`, colOp)
			} else {
				fmt.Fprintf(&b, ` AND %s?`, colOp)
			}
		}
	}

	where("path=", filter.Path)
	where("app_name=", filter.AppName)

	return b.String(), args
}

func scanCall(scanner RowScanner, call *models.FnCall) error {
	err := scanner.Scan(
		&call.ID,
		&call.CreatedAt,
		&call.StartedAt,
		&call.CompletedAt,
		&call.Status,
		&call.AppName,
		&call.Path,
	)

	if err == sql.ErrNoRows {
		return models.ErrCallNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// GetDatabase returns the underlying sqlx database implementation
func (ds *sqlStore) GetDatabase() *sqlx.DB {
	return ds.db
}
