package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/iron-io/functions/api/datastore/internal/datastoreutil"
	"github.com/iron-io/functions/api/models"
)

const routesTableCreate = `CREATE TABLE IF NOT EXISTS routes (
	app_name varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	maxc int NOT NULL,
	memory int NOT NULL,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	type varchar(16) NOT NULL,
	headers text NOT NULL,
	config text NOT NULL,
	PRIMARY KEY (app_name, path)
);`

const appsTableCreate = `CREATE TABLE IF NOT EXISTS apps (
    name varchar(256) NOT NULL PRIMARY KEY,
	config text NOT NULL
);`

const extrasTableCreate = `CREATE TABLE IF NOT EXISTS extras (
    id varchar(256) NOT NULL PRIMARY KEY,
	value varchar(256) NOT NULL
);`

const routeSelector = `SELECT app_name, path, image, format, maxc, memory, type, timeout, idle_timeout, headers, config FROM routes`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

type rowQuerier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

/*
MySQLDatastore defines a basic MySQL Datastore struct.
*/
type MySQLDatastore struct {
	db *sql.DB
}

/*
New creates a new MySQL Datastore.
*/
func New(url *url.URL) (models.Datastore, error) {
	u := fmt.Sprintf("%s@%s%s", url.User.String(), url.Host, url.Path)
	db, err := sql.Open("mysql", u)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	maxIdleConns := 30
	db.SetMaxIdleConns(maxIdleConns)
	logrus.WithFields(logrus.Fields{"max_idle_connections": maxIdleConns}).Info("MySQL dialed")

	pg := &MySQLDatastore{
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

/*
InsertApp inserts an app to MySQL.
*/
func (ds *MySQLDatastore) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	var cbyte []byte
	var err error

	if app.Config != nil {
		cbyte, err = json.Marshal(app.Config)
		if err != nil {
			return nil, err
		}
	}
	stmt, err := ds.db.Prepare("INSERT apps SET name=?,config=?")

	if err != nil {
		return nil, err
	}

	_, err = stmt.Exec(app.Name, string(cbyte))

	if err != nil {
		mysqlErr := err.(*mysql.MySQLError)
		if mysqlErr.Number == 1062 {
			return nil, models.ErrAppsAlreadyExists
		}
		return nil, err
	}

	return app, nil
}

/*
UpdateApp updates an existing app on MySQL.
*/
func (ds *MySQLDatastore) UpdateApp(ctx context.Context, newapp *models.App) (*models.App, error) {
	app := &models.App{Name: newapp.Name}
	err := ds.Tx(func(tx *sql.Tx) error {
		row := ds.db.QueryRow(`SELECT config FROM apps WHERE name=?`, app.Name)

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

		stmt, err := ds.db.Prepare(`UPDATE apps SET config=? WHERE name=?`)

		if err != nil {
			return err
		}

		res, err := stmt.Exec(string(cbyte), app.Name)

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

/*
RemoveApp removes an existing app on MySQL.
*/
func (ds *MySQLDatastore) RemoveApp(ctx context.Context, appName string) error {
	_, err := ds.db.Exec(`
	  DELETE FROM apps
	  WHERE name = ?
	`, appName)

	if err != nil {
		return err
	}

	return nil
}

/*
GetApp retrieves an app from MySQL.
*/
func (ds *MySQLDatastore) GetApp(ctx context.Context, name string) (*models.App, error) {
	row := ds.db.QueryRow(`SELECT name, config FROM apps WHERE name=?`, name)

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

/*
GetApps retrieves an array of apps according to a specific filter.
*/
func (ds *MySQLDatastore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}
	filterQuery, args := buildFilterAppQuery(filter)
	rows, err := ds.db.Query(fmt.Sprintf("SELECT DISTINCT name, config FROM apps %s", filterQuery), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var app models.App
		err := scanApp(rows, &app)

		if err != nil {
			break
		}
		res = append(res, &app)
	}

	if err := rows.Err(); err != nil {
		return res, err
	}
	return res, nil
}

/*
InsertRoute inserts an route to MySQL.
*/
func (ds *MySQLDatastore) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	hbyte, err := json.Marshal(route.Headers)
	if err != nil {
		return nil, err
	}

	cbyte, err := json.Marshal(route.Config)
	if err != nil {
		return nil, err
	}

	err = ds.Tx(func(tx *sql.Tx) error {
		r := tx.QueryRow(`SELECT 1 FROM apps WHERE name=?`, route.AppName)
		if err := r.Scan(new(int)); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			}
		}
		same, err := tx.Query(`SELECT 1 FROM routes WHERE app_name=? AND path=?`,
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
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

/*
UpdateRoute updates an existing route on MySQL.
*/
func (ds *MySQLDatastore) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	var route models.Route
	err := ds.Tx(func(tx *sql.Tx) error {
		row := ds.db.QueryRow(fmt.Sprintf("%s WHERE app_name=? AND path=?", routeSelector), newroute.AppName, newroute.Path)
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
			image = ?,
			format = ?,
			maxc = ?,
			memory = ?,
			type = ?,
			timeout = ?,
			idle_timeout = ?,
			headers = ?,
			config = ?
		WHERE app_name = ? AND path = ?;`,
			route.Image,
			route.Format,
			route.MaxConcurrency,
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
			return models.ErrRoutesNotFound
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return &route, nil
}

/*
RemoveRoute removes an existing route on MySQL.
*/
func (ds *MySQLDatastore) RemoveRoute(ctx context.Context, appName, routePath string) error {
	res, err := ds.db.Exec(`
		DELETE FROM routes
		WHERE path = ? AND app_name = ?
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

	if headerStr == "" {
		return models.ErrRoutesNotFound
	}

	if err := json.Unmarshal([]byte(headerStr), &route.Headers); err != nil {
		return err
	}
	return json.Unmarshal([]byte(configStr), &route.Config)
}

/*
GetRoute retrieves a route from MySQL.
*/
func (ds *MySQLDatastore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	var route models.Route

	row := ds.db.QueryRow(fmt.Sprintf("%s WHERE app_name=? AND path=?", routeSelector), appName, routePath)
	err := scanRoute(row, &route)

	if err == sql.ErrNoRows {
		return nil, models.ErrRoutesNotFound
	} else if err != nil {
		return nil, err
	}
	return &route, nil
}

/*
GetRoutes retrieves an array of routes according to a specific filter.
*/
func (ds *MySQLDatastore) GetRoutes(ctx context.Context, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	filterQuery, args := buildFilterRouteQuery(filter)
	rows, err := ds.db.Query(fmt.Sprintf("%s %s", routeSelector, filterQuery), args...)
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
func (ds *MySQLDatastore) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) ([]*models.Route, error) {
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
	rows, err := ds.db.Query(fmt.Sprintf("%s %s", routeSelector, filterQuery), args...)
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
		return "WHERE name LIKE ?", []interface{}{filter.Name}
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
				fmt.Fprintf(&b, "WHERE %s ?", colOp)
			} else {
				fmt.Fprintf(&b, " AND %s ?", colOp)
			}
		}
	}

	where("path =", filter.Path)
	where("app_name =", filter.AppName)
	where("image =", filter.Image)

	return b.String(), args
}

/*
Put inserts an extra into MySQL.
*/
func (ds *MySQLDatastore) Put(ctx context.Context, key, value []byte) error {
	_, err := ds.db.Exec(`
	    INSERT INTO extras (
			id,
			value
		)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
			value = ?
		`, string(key), string(value), string(value))

	if err != nil {
		return err
	}

	return nil
}

/*
Get retrieves the value of a specific extra from MySQL.
*/
func (ds *MySQLDatastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	row := ds.db.QueryRow("SELECT value FROM extras WHERE id=?", key)

	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return []byte(value), nil
}

/*
Tx Begins and commits a MySQL Transaction.
*/
func (ds *MySQLDatastore) Tx(f func(*sql.Tx) error) error {
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
