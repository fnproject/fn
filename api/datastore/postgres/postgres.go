package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/models"
	_ "github.com/lib/pq"
)

const routesTableCreate = `
CREATE TABLE IF NOT EXISTS routes (
	app_name character varying(256) NOT NULL,
	path text NOT NULL,
    image character varying(256) NOT NULL,
	headers text NOT NULL,
	PRIMARY KEY (app_name, path)
);`

const appsTableCreate = `CREATE TABLE IF NOT EXISTS apps (
    name character varying(256) NOT NULL PRIMARY KEY
);`

const extrasTableCreate = `CREATE TABLE IF NOT EXISTS extras (
    key character varying(256) NOT NULL PRIMARY KEY,
	value character varying(256) NOT NULL
);`

const routeSelector = `SELECT app_name, path, image, headers FROM routes`

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

func (ds *PostgresDatastore) StoreApp(app *models.App) (*models.App, error) {
	_, err := ds.db.Exec(`
	  INSERT INTO apps (name)
		VALUES ($1)
		ON CONFLICT (name) DO NOTHING
		RETURNING name;
	`, app.Name)
	// todo: after we support headers, the conflict should update the headers.

	if err != nil {
		return nil, err
	}

	return app, nil
}

func (ds *PostgresDatastore) RemoveApp(appName string) error {
	_, err := ds.db.Exec(`
	  DELETE FROM apps
	  WHERE name = $1
	`, appName)

	if err != nil {
		return err
	}

	return nil
}

func (ds *PostgresDatastore) GetApp(name string) (*models.App, error) {
	row := ds.db.QueryRow("SELECT name FROM apps WHERE name=$1", name)

	var resName string
	err := row.Scan(&resName)

	res := &models.App{
		Name: resName,
	}

	if err != nil {
		return nil, err
	}

	return res, nil
}

func scanApp(scanner rowScanner, app *models.App) error {
	err := scanner.Scan(
		&app.Name,
	)

	return err
}

func (ds *PostgresDatastore) GetApps(filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}

	rows, err := ds.db.Query(`
		SELECT DISTINCT *
		FROM apps`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var app models.App
		err := scanApp(rows, &app)

		if err != nil {
			return nil, err
		}
		res = append(res, &app)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *PostgresDatastore) StoreRoute(route *models.Route) (*models.Route, error) {
	var headers string

	hbyte, err := json.Marshal(route.Headers)
	if err != nil {
		return nil, err
	}

	headers = string(hbyte)

	_, err = ds.db.Exec(`
		INSERT INTO routes (
			app_name, 
			path, 
			image,
			headers
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (app_name, path) DO UPDATE SET
			path = $2,
			image = $3,
			headers = $4;
		`,
		route.AppName,
		route.Path,
		route.Image,
		headers,
	)

	if err != nil {
		return nil, err
	}
	return route, nil
}

func (ds *PostgresDatastore) RemoveRoute(appName, routeName string) error {
	_, err := ds.db.Exec(`
		DELETE FROM routes
		WHERE name = $1
	`, routeName)

	if err != nil {
		return err
	}
	return nil
}

func scanRoute(scanner rowScanner, route *models.Route) error {
	var headerStr string
	err := scanner.Scan(
		// &route.Name,
		&route.AppName,
		&route.Path,
		&route.Image,
		&headerStr,
	)

	if headerStr == "" {
		return models.ErrRoutesNotFound
	}

	err = json.Unmarshal([]byte(headerStr), &route.Headers)

	return err
}

func getRoute(qr rowQuerier, routeName string) (*models.Route, error) {
	var route models.Route

	row := qr.QueryRow(fmt.Sprintf("%s WHERE name=$1", routeSelector), routeName)
	err := scanRoute(row, &route)

	if err == sql.ErrNoRows {
		return nil, models.ErrRoutesNotFound
	} else if err != nil {
		return nil, err
	}
	return &route, nil
}

func (ds *PostgresDatastore) GetRoute(appName, routeName string) (*models.Route, error) {
	return getRoute(ds.db, routeName)
}

func (ds *PostgresDatastore) GetRoutes(filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	filterQuery := buildFilterQuery(filter)
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
			return nil, err
		}
		res = append(res, &route)

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *PostgresDatastore) GetRoutesByApp(appName string, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	filter.AppName = appName
	filterQuery := buildFilterQuery(filter)
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
			return nil, err
		}
		res = append(res, &route)

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func buildFilterQuery(filter *models.RouteFilter) string {
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

func (ds *PostgresDatastore) Put(key, value []byte) error {
	_, err := ds.db.Exec(`
	    INSERT INTO extras (
			key,
			value
		)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET
			value = $1;
		`, value)

	if err != nil {
		return err
	}

	return nil
}

func (ds *PostgresDatastore) Get(key []byte) ([]byte, error) {
	row := ds.db.QueryRow("SELECT value FROM extras WHERE key=$1", key)

	var value []byte
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return value, nil
}
