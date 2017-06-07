package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"gitlab-odx.oracle.com/odx/functions/api/datastore/internal/datastoreutil"
	"gitlab-odx.oracle.com/odx/functions/api/models"
)

const routesTableCreate = `CREATE TABLE IF NOT EXISTS routes (
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
);`

const appsTableCreate = `CREATE TABLE IF NOT EXISTS apps (
    name varchar(256) NOT NULL PRIMARY KEY,
	config text NOT NULL
);`

const extrasTableCreate = `CREATE TABLE IF NOT EXISTS extras (
    id varchar(256) NOT NULL PRIMARY KEY,
	value varchar(256) NOT NULL
);`

const routeSelector = `SELECT app_name, path, image, format, memory, type, timeout, idle_timeout, headers, config FROM routes`

const callTableCreate = `CREATE TABLE IF NOT EXISTS calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_name varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	PRIMARY KEY (id)
);`

const callSelector = `SELECT id, created_at, started_at, completed_at, status, app_name, path FROM calls`

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
	tables := []string{routesTableCreate, appsTableCreate, extrasTableCreate, callTableCreate}
	dialect := "mysql"
	sqlDatastore := &MySQLDatastore{}
	dataSourceName := fmt.Sprintf("%s@%s%s", url.User.String(), url.Host, url.Path)

	db, err := datastoreutil.NewDatastore(dataSourceName, dialect, tables)
	if err != nil {
		return nil, err
	}

	sqlDatastore.db = db
	return datastoreutil.NewValidator(sqlDatastore), nil

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

	return err
}

/*
GetApp retrieves an app from MySQL.
*/
func (ds *MySQLDatastore) GetApp(ctx context.Context, name string) (*models.App, error) {
	queryStr := `SELECT name, config FROM apps WHERE name=?`
	queryArgs := []interface{}{name}
	return datastoreutil.SQLGetApp(ds.db, queryStr, queryArgs...)
}

/*
GetApps retrieves an array of apps according to a specific filter.
*/
func (ds *MySQLDatastore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	whereStm := "WHERE name LIKE ?"
	selectStm := "SELECT DISTINCT name, config FROM apps %s"

	return datastoreutil.SQLGetApps(ds.db, filter, whereStm, selectStm)
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
		if err := datastoreutil.ScanRoute(row, &route); err == sql.ErrNoRows {
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
			memory = ?,
			type = ?,
			timeout = ?,
			idle_timeout = ?,
			headers = ?,
			config = ?
		WHERE app_name = ? AND path = ?;`,
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
	deleteStm := `DELETE FROM routes WHERE path = ? AND app_name = ?`
	return datastoreutil.SQLRemoveRoute(ds.db, appName, routePath, deleteStm)
}

/*
GetRoute retrieves a route from MySQL.
*/
func (ds *MySQLDatastore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	rSelectCondition := "%s WHERE app_name=? AND path=?"

	return datastoreutil.SQLGetRoute(ds.db, appName, routePath, rSelectCondition, routeSelector)
}

/*
GetRoutes retrieves an array of routes according to a specific filter.
*/
func (ds *MySQLDatastore) GetRoutes(ctx context.Context, filter *models.RouteFilter) ([]*models.Route, error) {
	whereStm := "WHERE %s ?"
	andStm := " AND %s ?"

	return datastoreutil.SQLGetRoutes(ds.db, filter, routeSelector, whereStm, andStm)
}

/*
GetRoutesByApp retrieves a route with a specific app name.
*/
func (ds *MySQLDatastore) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) ([]*models.Route, error) {
	whereStm := "WHERE %s ?"
	andStm := " AND %s ?"
	defaultFilterQuery := "WHERE app_name = ?"

	return datastoreutil.SQLGetRoutesByApp(ds.db, appName, filter, routeSelector, defaultFilterQuery, whereStm, andStm)
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

func (ds *MySQLDatastore) InsertTask(ctx context.Context, task *models.Task) error {
	stmt, err := ds.db.Prepare("INSERT calls SET id=?,created_at=?,started_at=?,completed_at=?,status=?,app_name=?,path=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(task.ID, task.CreatedAt.String(),
		task.StartedAt.String(), task.CompletedAt.String(),
		task.Status, task.AppName, task.Path)

	if err != nil {
		return err
	}

	return nil
}

func (ds *MySQLDatastore) GetTask(ctx context.Context, callID string) (*models.FnCall, error) {
	whereStm := "%s WHERE id=?"

	return datastoreutil.SQLGetCall(ds.db, callSelector, callID, whereStm)
}

func (ds *MySQLDatastore) GetTasks(ctx context.Context, filter *models.CallFilter) (models.FnCalls, error) {
	whereStm := "WHERE %s ?"
	andStm := " AND %s ?"

	return datastoreutil.SQLGetCalls(ds.db, callSelector, filter, whereStm, andStm)
}
