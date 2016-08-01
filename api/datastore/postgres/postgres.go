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
	name character varying(256) NOT NULL PRIMARY KEY,
	path text NOT NULL,
    app_name character varying(256) NOT NULL,
    image character varying(256) NOT NULL,
	headers text NOT NULL
);`

const appsTableCreate = `CREATE TABLE IF NOT EXISTS apps (
    name character varying(256) NOT NULL PRIMARY KEY
);`

const routeSelector = `SELECT name, path, app_name, image, headers FROM routes`

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

	for _, v := range []string{routesTableCreate, appsTableCreate} {
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
		RETURNING name;
	`, app.Name)

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
			name, app_name, path, image,
			headers
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (name) DO UPDATE SET
			path = $3,
			image = $4,
			headers = $5;
		`,
		route.Name,
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
		&route.Name,
		&route.Path,
		&route.AppName,
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

	for i, field := range filterQueries {
		if i == 0 {
			filterQuery = fmt.Sprintf("WHERE %s ", field)
		} else {
			filterQuery = fmt.Sprintf("%s AND %s", filterQuery, field)
		}
	}

	return filterQuery
}
