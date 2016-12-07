package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"context"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/models"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

const routesTableCreate = `
CREATE TABLE IF NOT EXISTS routes (
	app_name character varying(256) NOT NULL,
	path text NOT NULL,
	image character varying(256) NOT NULL,
	format character varying(16) NOT NULL,
	maxc integer NOT NULL,
	memory integer NOT NULL,
	timeout integer NOT NULL,
	type character varying(16) NOT NULL,
	headers text NOT NULL,
	config text NOT NULL,
	PRIMARY KEY (app_name, path)
);`

const appsTableCreate = `CREATE TABLE IF NOT EXISTS apps (
    name character varying(256) NOT NULL PRIMARY KEY,
	config text NOT NULL
);`

const extrasTableCreate = `CREATE TABLE IF NOT EXISTS extras (
    key character varying(256) NOT NULL PRIMARY KEY,
	value character varying(256) NOT NULL
);`

const routeSelector = `SELECT app_name, path, image, format, maxc, memory, type, timeout, headers, config FROM routes`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

type rowQuerier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

type PostgresDatastore struct {
	db *sql.DB
}

func New(url *url.URL) (models.Datastore, error) {
	db, err := sql.Open("postgres", url.String())
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	maxIdleConns := 30 // c.MaxIdleConnections
	db.SetMaxIdleConns(maxIdleConns)
	logrus.WithFields(logrus.Fields{"max_idle_connections": maxIdleConns}).Info("Postgres dialed")

	pg := &PostgresDatastore{
		db: db,
	}

	for _, v := range []string{routesTableCreate, appsTableCreate, extrasTableCreate} {
		_, err = db.Exec(v)
		if err != nil {
			return nil, err
		}
	}

	return pg, nil
}

func (ds *PostgresDatastore) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	var cbyte []byte
	var err error

	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}

	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if app.Config != nil {
		cbyte, err = json.Marshal(app.Config)
		if err != nil {
			return nil, err
		}
	}

	_, err = ds.db.Exec(`INSERT INTO apps (name, config) VALUES ($1, $2);`,
		app.Name,
		string(cbyte),
	)

	if err != nil {
		pqErr := err.(*pq.Error)
		if pqErr.Code == "23505" {
			return nil, models.ErrAppsAlreadyExists
		}
		return nil, err
	}

	return app, nil
}

func (ds *PostgresDatastore) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrAppsNotFound
	}

	cbyte, err := json.Marshal(app.Config)
	if err != nil {
		return nil, err
	}

	res, err := ds.db.Exec(`
	  UPDATE apps SET
		config = $2
	  WHERE name = $1
	  RETURNING *;
	`,
		app.Name,
		string(cbyte),
	)

	if err != nil {
		return nil, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if n == 0 {
		return nil, models.ErrAppsNotFound
	}

	return app, nil
}

func (ds *PostgresDatastore) RemoveApp(ctx context.Context, appName string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}

	_, err := ds.db.Exec(`
	  DELETE FROM apps
	  WHERE name = $1
	`, appName)

	if err != nil {
		return err
	}

	return nil
}

func (ds *PostgresDatastore) GetApp(ctx context.Context, name string) (*models.App, error) {
	if name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	row := ds.db.QueryRow("SELECT name, config FROM apps WHERE name=$1", name)

	var resName string
	var config string
	err := row.Scan(&resName, &config)

	res := &models.App{
		Name: resName,
	}

	json.Unmarshal([]byte(config), &res.Config)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrAppsNotFound
		}
		return nil, err
	}

	return res, nil
}

func scanApp(scanner rowScanner, app *models.App) error {
	var configStr string

	err := scanner.Scan(
		&app.Name,
		&configStr,
	)

	json.Unmarshal([]byte(configStr), &app.Config)

	return err
}

func (ds *PostgresDatastore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}

	filterQuery := buildFilterAppQuery(filter)
	rows, err := ds.db.Query(fmt.Sprintf("SELECT DISTINCT * FROM apps %s", filterQuery))

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

func (ds *PostgresDatastore) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	hbyte, err := json.Marshal(route.Headers)
	if err != nil {
		return nil, err
	}

	cbyte, err := json.Marshal(route.Config)
	if err != nil {
		return nil, err
	}

	_, err = ds.db.Exec(`
		INSERT INTO routes (
			app_name,
			path,
			image,
			format,
			maxc,
			memory,
			type,
			timeout,
			headers,
			config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);`,
		route.AppName,
		route.Path,
		route.Image,
		route.Format,
		route.MaxConcurrency,
		route.Memory,
		route.Type,
		route.Timeout,
		string(hbyte),
		string(cbyte),
	)

	if err != nil {
		pqErr := err.(*pq.Error)
		if pqErr.Code == "23505" {
			return nil, models.ErrRoutesAlreadyExists
		}
		return nil, err
	}
	return route, nil
}

func (ds *PostgresDatastore) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	hbyte, err := json.Marshal(route.Headers)
	if err != nil {
		return nil, err
	}

	cbyte, err := json.Marshal(route.Config)
	if err != nil {
		return nil, err
	}

	res, err := ds.db.Exec(`
		UPDATE routes SET
			image = $3,
			format = $4,
			memory = $5,
			maxc = $6,
			type = $7,
			timeout = $8,
			headers = $9,
			config = $10
		WHERE app_name = $1 AND path = $2;`,
		route.AppName,
		route.Path,
		route.Image,
		route.Format,
		route.Memory,
		route.MaxConcurrency,
		route.Type,
		route.Timeout,
		string(hbyte),
		string(cbyte),
	)

	if err != nil {
		return nil, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if n == 0 {
		return nil, models.ErrRoutesNotFound
	}

	return route, nil
}

func (ds *PostgresDatastore) RemoveRoute(ctx context.Context, appName, routePath string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}

	if routePath == "" {
		return models.ErrDatastoreEmptyRoutePath
	}

	res, err := ds.db.Exec(`
		DELETE FROM routes
		WHERE path = $1 AND app_name = $2
	`, routePath, appName)

	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return models.ErrRoutesRemoving
	}

	return nil
}

func scanRoute(scanner rowScanner, route *models.Route) error {
	var headerStr string
	var configStr string

	err := scanner.Scan(
		&route.AppName,
		&route.Path,
		&route.Image,
		&route.Format,
		&route.Memory,
		&route.MaxConcurrency,
		&route.Type,
		&route.Timeout,
		&headerStr,
		&configStr,
	)

	if headerStr == "" {
		return models.ErrRoutesNotFound
	}

	json.Unmarshal([]byte(headerStr), &route.Headers)
	json.Unmarshal([]byte(configStr), &route.Config)

	return err
}

func (ds *PostgresDatastore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if routePath == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	var route models.Route

	row := ds.db.QueryRow(fmt.Sprintf("%s WHERE app_name=$1 AND path=$2", routeSelector), appName, routePath)
	err := scanRoute(row, &route)

	if err == sql.ErrNoRows {
		return nil, models.ErrRoutesNotFound
	} else if err != nil {
		return nil, err
	}
	return &route, nil
}

func (ds *PostgresDatastore) GetRoutes(ctx context.Context, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	filterQuery := buildFilterRouteQuery(filter)
	rows, err := ds.db.Query(fmt.Sprintf("%s %s", routeSelector, filterQuery))
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

func (ds *PostgresDatastore) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	filter.AppName = appName
	filterQuery := buildFilterRouteQuery(filter)
	rows, err := ds.db.Query(fmt.Sprintf("%s %s", routeSelector, filterQuery))
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

func buildFilterAppQuery(filter *models.AppFilter) string {
	filterQuery := ""

	if filter != nil {
		filterQueries := []string{}
		if filter.Name != "" {
			filterQueries = append(filterQueries, fmt.Sprintf("name LIKE '%s'", filter.Name))
		}

		for i, field := range filterQueries {
			if i == 0 {
				filterQuery = fmt.Sprintf("WHERE %s ", field)
			} else {
				filterQuery = fmt.Sprintf("%s AND %s", filterQuery, field)
			}
		}
	}

	return filterQuery
}

func buildFilterRouteQuery(filter *models.RouteFilter) string {
	filterQuery := ""

	filterQueries := []string{}
	if filter.Path != "" {
		filterQueries = append(filterQueries, fmt.Sprintf("path = '%s'", filter.Path))
	}

	if filter.AppName != "" {
		filterQueries = append(filterQueries, fmt.Sprintf("app_name = '%s'", filter.AppName))
	}

	if filter.Image != "" {
		filterQueries = append(filterQueries, fmt.Sprintf("image = '%s'", filter.Image))
	}

	for i, field := range filterQueries {
		if i == 0 {
			filterQuery = fmt.Sprintf("WHERE %s ", field)
		} else {
			filterQuery = fmt.Sprintf("%s AND %s", filterQuery, field)
		}
	}

	return filterQuery
}

func (ds *PostgresDatastore) Put(ctx context.Context, key, value []byte) error {
	if key == nil || len(key) == 0 {
		return models.ErrDatastoreEmptyKey
	}

	_, err := ds.db.Exec(`
	    INSERT INTO extras (
			key,
			value
		)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET
			value = $2;
		`, string(key), string(value))

	if err != nil {
		return err
	}

	return nil
}

func (ds *PostgresDatastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	if key == nil || len(key) == 0 {
		return nil, models.ErrDatastoreEmptyKey
	}

	row := ds.db.QueryRow("SELECT value FROM extras WHERE key=$1", key)

	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return []byte(value), nil
}
