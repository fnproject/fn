package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"context"

	"bytes"
	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/datastore/internal/datastoreutil"
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
	idle_timeout integer NOT NULL,
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

const routeSelector = `SELECT app_name, path, image, format, maxc, memory, type, timeout, idle_timeout, headers, config FROM routes`

type rowScanner interface {
	Scan(dest ...interface{}) error
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

	return datastoreutil.NewValidator(pg), nil
}

func (ds *PostgresDatastore) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	var cbyte []byte
	var err error

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

func (ds *PostgresDatastore) UpdateApp(ctx context.Context, newapp *models.App) (*models.App, error) {
	app := &models.App{Name: newapp.Name}
	err := ds.Tx(func(tx *sql.Tx) error {
		row := ds.db.QueryRow("SELECT config FROM apps WHERE name=$1", app.Name)

		var config string
		if err := row.Scan(&config); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			}
			return err
		}

		if len(config) > 0 {
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

		res, err := ds.db.Exec(`UPDATE apps SET config = $2 WHERE name = $1;`, app.Name, string(cbyte))
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

func (ds *PostgresDatastore) RemoveApp(ctx context.Context, appName string) error {
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
	row := ds.db.QueryRow("SELECT name, config FROM apps WHERE name=$1", name)

	var resName string
	var config string
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

func scanApp(scanner rowScanner, app *models.App) error {
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

func (ds *PostgresDatastore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}

	filterQuery, args := buildFilterAppQuery(filter)
	rows, err := ds.db.Query(fmt.Sprintf("SELECT DISTINCT * FROM apps %s", filterQuery), args...)
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
	hbyte, err := json.Marshal(route.Headers)
	if err != nil {
		return nil, err
	}

	cbyte, err := json.Marshal(route.Config)
	if err != nil {
		return nil, err
	}

	err = ds.Tx(func(tx *sql.Tx) error {
		r := tx.QueryRow(`SELECT 1 FROM apps WHERE name=$1`, route.AppName)
		if err := r.Scan(new(int)); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			}
			return err
		}

		same, err := tx.Query(`SELECT 1 FROM routes WHERE app_name=$1 AND path=$2`,
			route.AppName, route.Path)
		if err != nil {
			return err
		}
		defer same.Close()
		if same.Next() {
			return models.ErrRoutesAlreadyExists
		}

		_, err = tx.Exec(`
		INSERT INTO routes (
			app_name,
			path,
			image,
			format,
			maxc,
			memory,
			type,
			timeout,
			idle_timeout,
			headers,
			config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);`,
			route.AppName,
			route.Path,
			route.Image,
			route.Format,
			route.MaxConcurrency,
			route.Memory,
			route.Type,
			route.Timeout,
			route.IdleTimeout,
			string(hbyte),
			string(cbyte),
		)
		return err
	})

	if err != nil {
		return nil, err
	}
	return route, nil
}

func (ds *PostgresDatastore) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	var route models.Route
	err := ds.Tx(func(tx *sql.Tx) error {
		row := ds.db.QueryRow(fmt.Sprintf("%s WHERE app_name=$1 AND path=$2", routeSelector), newroute.AppName, newroute.Path)
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

		res, err := tx.Exec(`
		UPDATE routes SET
			image = $3,
			format = $4,
			maxc = $5,
			memory = $6,
			type = $7,
			timeout = $8,
			idle_timeout = $9,
			headers = $10,
			config = $11
		WHERE app_name = $1 AND path = $2;`,
			route.AppName,
			route.Path,
			route.Image,
			route.Format,
			route.MaxConcurrency,
			route.Memory,
			route.Type,
			route.Timeout,
			route.IdleTimeout,
			string(hbyte),
			string(cbyte),
		)

		if err != nil {
			return err
		}

		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return models.ErrRoutesNotFound
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (ds *PostgresDatastore) RemoveRoute(ctx context.Context, appName, routePath string) error {
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
		&route.MaxConcurrency,
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

func (ds *PostgresDatastore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
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
	filterQuery, args := buildFilterRouteQuery(filter)
	rows, err := ds.db.Query(fmt.Sprintf("%s %s", routeSelector, filterQuery), args...)
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

	var filterQuery string
	var args []interface{}
	if filter == nil {
		filterQuery = "WHERE app_name = $1"
		args = []interface{}{appName}
	} else {
		filter.AppName = appName
		filterQuery, args = buildFilterRouteQuery(filter)
	}
	rows, err := ds.db.Query(fmt.Sprintf("%s %s", routeSelector, filterQuery), args...)
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
func buildFilterAppQuery(filter *models.AppFilter) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}

	if filter.Name != "" {
		return "WHERE name LIKE $1", []interface{}{filter.Name}
	}

	return "", nil
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
				fmt.Fprintf(&b, "WHERE %s $1", colOp)
			} else {
				fmt.Fprintf(&b, " AND %s $%d", colOp, len(args))
			}
		}
	}

	where("path =", filter.Path)
	where("app_name =", filter.AppName)
	where("image =", filter.Image)

	return b.String(), args
}

func (ds *PostgresDatastore) Put(ctx context.Context, key, value []byte) error {
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

func (ds *PostgresDatastore) Tx(f func(*sql.Tx) error) error {
	tx, err := ds.db.Begin()
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
