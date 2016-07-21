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

const routesTableCreate = `CREATE TABLE IF NOT EXISTS routes (
	name  character varying(256)  NOT NULL PRIMARY KEY,
	path text NOT NULL,
    app_name  character varying(256)  NOT NULL,
    image character varying(256) NOT NULL,
    type  character varying(256)  NOT NULL,
	container_path text NOT NULL,
	headers text NOT NULL,
);`

const appsTableCreate = `CREATE TABLE IF NOT EXISTS apps (
    name character varying(256) NOT NULL PRIMARY KEY,
);`

const routeSelector = `SELECT path, app_name, image, type, container_path, headers FROM routes`

// Tries to read in properties into `route` from `scanner`. Bubbles up errors.
// Capture sql.Row and sql.Rows
type rowScanner interface {
	Scan(dest ...interface{}) error
}

// Capture sql.DB and sql.Tx
type rowQuerier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func scanRoute(scanner rowScanner, route *models.Route) error {
	err := scanner.Scan(
		&route.Name,
		&route.Path,
		&route.AppName,
		&route.Image,
		&route.Type,
		&route.ContainerPath,
		&route.Headers,
	)
	return err
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

func (ds *PostgresDatastore) StoreRoute(route *models.Route) (*models.Route, error) {
	var headers string

	hbyte, err := json.Marshal(route.Headers)
	if err != nil {
		return nil, err
	}

	headers = string(hbyte)

	err = ds.db.QueryRow(`
		INSERT INTO routes (
			name, app_name, path, image,
			type, container_path, headers
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		route.Name,
		route.AppName,
		route.Path,
		route.Image,
		route.Type,
		route.ContainerPath,
		headers,
	).Scan(nil)
	if err != nil {
		return nil, err
	}
	return route, nil
}

func (ds *PostgresDatastore) RemoveRoute(appName, routeName string) error {
	err := ds.db.QueryRow(`
		DELETE FROM routes
		WHERE name = $1`,
		routeName,
	).Scan(nil)
	if err != nil {
		return err
	}
	return nil
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

// TODO: Add pagination
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

func (ds *PostgresDatastore) GetApp(name string) (*models.App, error) {
	row := ds.db.QueryRow("SELECT * FROM groups WHERE name=$1", name)

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

func (ds *PostgresDatastore) GetApps(filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}

	rows, err := ds.db.Query(`
		SELECT DISTINCT *
		 FROM apps
		ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var app models.App
		err := rows.Scan(&app)

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

func (ds *PostgresDatastore) StoreApp(app *models.App) (*models.App, error) {
	err := ds.db.QueryRow(`
	  INSERT INTO apps (name)
		VALUES ($1)
	`, app.Name).Scan(&app.Name)
	// ON CONFLICT (name) DO UPDATE SET created_at = $2;

	if err != nil {
		return nil, err
	}

	return app, nil
}

func (ds *PostgresDatastore) RemoveApp(appName string) error {
	err := ds.db.QueryRow(`
	  DELETE FROM apps
	  WHERE name = $1
	`, appName).Scan(nil)

	if err != nil {
		return err
	}

	return nil
}

func buildFilterQuery(filter *models.RouteFilter) string {
	filterQuery := ""

	filterQueries := []string{}
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
