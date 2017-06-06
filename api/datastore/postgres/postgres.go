package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"context"

	"gitlab-odx.oracle.com/odx/functions/api/datastore/internal/datastoreutil"
	"gitlab-odx.oracle.com/odx/functions/api/models"
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

const callsTableCreate = `CREATE TABLE IF NOT EXISTS calls (
	created_at character varying(256) NOT NULL,
	started_at character varying(256) NOT NULL,
	completed_at character varying(256) NOT NULL,
	status character varying(256) NOT NULL,
	id character varying(256) NOT NULL,
	app_name character varying(256) NOT NULL,
	path character varying(256) NOT NULL,
	PRIMARY KEY (id)
);`

const callSelector = `SELECT id, created_at, started_at, completed_at, status, app_name, path FROM calls`

type PostgresDatastore struct {
	db *sql.DB
}

func New(url *url.URL) (models.Datastore, error) {
	tables := []string{routesTableCreate, appsTableCreate, extrasTableCreate, callsTableCreate}
	sqlDatastore := &PostgresDatastore{}
	dialect := "postgres"

	db, err := datastoreutil.NewDatastore(url.String(), dialect, tables)
	if err != nil {
		return nil, err
	}

	sqlDatastore.db = db
	return datastoreutil.NewValidator(sqlDatastore), nil
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
	_, err := ds.db.Exec(`DELETE FROM apps WHERE name = $1`, appName)

	return err
}

func (ds *PostgresDatastore) GetApp(ctx context.Context, name string) (*models.App, error) {
	queryStr := "SELECT name, config FROM apps WHERE name=$1"
	queryArgs := []interface{}{name}

	return datastoreutil.SQLGetApp(ds.db, queryStr, queryArgs...)
}

func (ds *PostgresDatastore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	whereStm := "WHERE name LIKE $1"
	selectStm := "SELECT DISTINCT * FROM apps %s"

	return datastoreutil.SQLGetApps(ds.db, filter, whereStm, selectStm)
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
	deleteStm := `DELETE FROM routes WHERE path = $1 AND app_name = $2`

	return datastoreutil.SQLRemoveRoute(ds.db, appName, routePath, deleteStm)
}

func (ds *PostgresDatastore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	rSelectCondition := "%s WHERE app_name=$1 AND path=$2"

	return datastoreutil.SQLGetRoute(ds.db, appName, routePath, rSelectCondition, routeSelector)
}

func (ds *PostgresDatastore) GetRoutes(ctx context.Context, filter *models.RouteFilter) ([]*models.Route, error) {
	whereStm := "WHERE %s $1"
	andStm := " AND %s $%d"

	return datastoreutil.SQLGetRoutes(ds.db, filter, routeSelector, whereStm, andStm)
}

func (ds *PostgresDatastore) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) ([]*models.Route, error) {
	defaultFilterQuery := "WHERE app_name = $1"
	whereStm := "WHERE %s $1"
	andStm := " AND %s $%d"

	return datastoreutil.SQLGetRoutesByApp(ds.db, appName, filter, routeSelector, defaultFilterQuery, whereStm, andStm)
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

func (ds *PostgresDatastore) InsertTask(ctx context.Context, task *models.Task) error {
	err := ds.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO calls (
				id,
				created_at,
				started_at,
				completed_at,
				status,
				app_name,
				path) VALUES ($1, $2, $3, $4, $5, $6, $7);`,
			task.ID,
			task.CreatedAt.String(),
			task.StartedAt.String(),
			task.CompletedAt.String(),
			task.Status,
			task.AppName,
			task.Path,
		)
		return err
	})
	return err
}

func (ds *PostgresDatastore) GetTask(ctx context.Context, callID string) (*models.FnCall, error) {
	whereStm := "%s WHERE id=$1"

	return datastoreutil.SQLGetCall(ds.db, callSelector, callID, whereStm)
}

func (ds *PostgresDatastore) GetTasks(ctx context.Context, filter *models.CallFilter) (models.FnCalls, error) {
	whereStm := "WHERE %s $1"
	andStm := " AND %s $2"

	return datastoreutil.SQLGetCalls(ds.db, callSelector, filter, whereStm, andStm)
}
