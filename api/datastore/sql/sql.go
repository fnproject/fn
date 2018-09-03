package sql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/fnproject/fn/api/datastore/sql/migrations"
	"github.com/fnproject/fn/api/models"
	"github.com/jmoiron/sqlx"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/datastore/sql/dbhelper"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/sirupsen/logrus"
)

// this aims to be an ANSI-SQL compliant package that uses only question
// mark syntax for var placement, leaning on sqlx to make compatible all
// queries to the actual underlying datastore.
//
// currently tested and working are postgres, mysql and sqlite3.

// TODO routes.created_at should be varchar(256), mysql will store 'text'
// fields not contiguous with other fields and this field is a fixed size,
// we'll get better locality with varchar. it's not terribly easy to do this
// with migrations (sadly, need complex transaction)

var tables = [...]string{`CREATE TABLE IF NOT EXISTS routes (
	app_id varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	memory int NOT NULL,
	cpus int,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	tmpfs_size int,
	type varchar(16) NOT NULL,
	headers text NOT NULL,
	config text NOT NULL,
	annotations text NOT NULL,
	created_at text,
	updated_at varchar(256),
	PRIMARY KEY (app_id, path)
);`,

	`CREATE TABLE IF NOT EXISTS apps (
	id varchar(256) NOT NULL PRIMARY KEY,
	name varchar(256) NOT NULL UNIQUE,
	config text NOT NULL,
	annotations text NOT NULL,
	syslog_url text,
	created_at varchar(256),
	updated_at varchar(256)
);`,

	`CREATE TABLE IF NOT EXISTS calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_id varchar(256) NOT NULL,
	fn_id varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	stats text,
	error text,
	PRIMARY KEY (id)
);`,

	`CREATE TABLE IF NOT EXISTS triggers (
	id varchar(256) NOT NULL PRIMARY KEY,
	name varchar(256) NOT NULL,
	app_id varchar(256) NOT NULL,
	fn_id varchar(256) NOT NULL,
	created_at varchar(256) NOT NULL,
	updated_at varchar(256) NOT NULL,
	type varchar(256) NOT NULL,
	source varchar(256) NOT NULL,
    annotations text NOT NULL,
    CONSTRAINT name_app_id_fn_id_unique UNIQUE (app_id, fn_id, name)
);`,

	`CREATE TABLE IF NOT EXISTS logs (
	id varchar(256) NOT NULL PRIMARY KEY,
	app_id varchar(256) NOT NULL,
	fn_id varchar(256) NOT NULL,
	call_id varchar(256) NOT NULL,
	log text NOT NULL
);`,

	`CREATE TABLE IF NOT EXISTS fns (
	id varchar(256) NOT NULL PRIMARY KEY,
	name varchar(256) NOT NULL,
	app_id varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	memory int NOT NULL,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	config text NOT NULL,
	annotations text NOT NULL,
	created_at varchar(256) NOT NULL,
	updated_at varchar(256) NOT NULL,
    CONSTRAINT name_app_id_unique UNIQUE (app_id, name)
);`,
}

const (
	routeSelector     = `SELECT app_id, path, image, format, memory, type, cpus, timeout, idle_timeout, tmpfs_size, headers, config, annotations, created_at, updated_at FROM routes`
	callSelector      = `SELECT id, created_at, started_at, completed_at, status, app_id, fn_id, path, stats, error FROM calls`
	appIDSelector     = `SELECT id, name, config, annotations, syslog_url, created_at, updated_at FROM apps WHERE id=?`
	ensureAppSelector = `SELECT id FROM apps WHERE name=?`

	fnSelector   = `SELECT id,name,app_id,image,format,memory,timeout,idle_timeout,config,annotations,created_at,updated_at FROM fns`
	fnIDSelector = fnSelector + ` WHERE id=?`

	triggerSelector   = `SELECT id,name,app_id,fn_id,type,source,annotations,created_at,updated_at FROM triggers`
	triggerIDSelector = triggerSelector + ` WHERE id=?`

	triggerIDSourceSelector = triggerSelector + ` WHERE app_id=? AND type=? AND source=?`

	EnvDBPingMaxRetries = "FN_DS_DB_PING_MAX_RETRIES"
)

var ( // compiler will yell nice things about our upbringing as a child
	_ models.Datastore = new(SQLStore)
	_ models.LogStore  = new(SQLStore)
)

type SQLStore struct {
	helper dbhelper.Helper
	db     *sqlx.DB
}

type sqlDsProvider int

// New will open the db specified by url, create any tables necessary
// and return a models.Datastore safe for concurrent usage.
func New(ctx context.Context, u *url.URL) (*SQLStore, error) {
	return newDS(ctx, u)
}

func (sqlDsProvider) Supports(u *url.URL) bool {
	_, ok := dbhelper.GetHelper(u.Scheme)
	return ok
}

func (sqlDsProvider) New(ctx context.Context, u *url.URL) (models.Datastore, error) {
	return newDS(ctx, u)
}

func (sqlDsProvider) String() string {
	return "sql"
}

type sqlLogsProvider int

func (sqlLogsProvider) String() string {
	return "sql"
}

func (sqlLogsProvider) Supports(u *url.URL) bool {
	_, ok := dbhelper.GetHelper(u.Scheme)
	return ok
}

func (sqlLogsProvider) New(ctx context.Context, u *url.URL) (models.LogStore, error) {
	return newDS(ctx, u)
}

// for test methods, return concrete type, but don't expose
func newDS(ctx context.Context, url *url.URL) (*SQLStore, error) {
	driver := url.Scheme

	log := common.Logger(ctx).WithFields(logrus.Fields{"url": common.MaskPassword(url)})
	helper, ok := dbhelper.GetHelper(driver)

	if !ok {
		return nil, fmt.Errorf("DB helper '%s' is not supported", driver)
	}

	uri, err := helper.PreConnect(url)

	if err != nil {
		return nil, fmt.Errorf("failed to initialise db helper %s : %s", driver, err)
	}

	log.WithFields(logrus.Fields{"url": uri}).Info("Connecting to DB")

	sqldb, err := sql.Open(driver, uri)
	if err != nil {
		log.WithError(err).Error("couldn't open db")
		return nil, err
	}

	db := sqlx.NewDb(sqldb, driver)

	// force a connection and test that it worked
	err = pingWithRetry(ctx, db)
	if err != nil {
		log.WithError(err).Error("couldn't ping db")
		return nil, err
	}

	maxIdleConns := 256 // TODO we need to strip this out of the URL probably
	db.SetMaxIdleConns(maxIdleConns)
	log.WithFields(logrus.Fields{"max_idle_connections": maxIdleConns, "datastore": driver}).Info("datastore dialed")

	db, err = helper.PostCreate(db)
	if err != nil {
		log.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't initialize db")
		return nil, err
	}
	sdb := &SQLStore{db: db, helper: helper}

	// NOTE: runMigrations happens before we create all the tables, so that it
	// can detect whether the db did not exist and insert the latest version of
	// the migrations BEFORE the tables are created (it uses table info to
	// determine that).
	//
	// we either create all the tables with the latest version of the schema,
	// insert the latest version to the migration table and bail without running
	// any migrations.
	// OR
	// run all migrations necessary to get up to the latest, inserting that version,
	// [and the tables exist so CREATE IF NOT EXIST guards us when we run the create queries].
	err = sdb.Tx(func(tx *sqlx.Tx) error {
		err = sdb.runMigrations(ctx, tx, migrations.Migrations)
		if err != nil {
			log.WithError(err).Error("error running migrations")
			return err
		}

		for _, v := range tables {
			_, err = tx.ExecContext(ctx, v)
			if err != nil {
				log.WithError(err).Error("error creating tables")
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return sdb, nil
}

func pingWithRetry(ctx context.Context, db *sqlx.DB) (err error) {

	attempts := int64(10)
	if tmp := os.Getenv(EnvDBPingMaxRetries); tmp != "" {
		attempts, err = strconv.ParseInt(tmp, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse (%s) invalid %s=%s", err, EnvDBPingMaxRetries, tmp)
		}
		if attempts < 0 {
			return fmt.Errorf("cannot parse invalid %s=%s", EnvDBPingMaxRetries, tmp)
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	for i := int64(0); i < attempts; i++ {
		err = db.PingContext(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * 1):
		}
	}
	return err
}

// check if the db already existed, if the db is brand new then we can skip
// over all the migrations BUT we must be sure to set the right migration
// number so that only current migrations are skipped, not any future ones.
func (ds *SQLStore) runMigrations(ctx context.Context, tx *sqlx.Tx, migrations []migratex.Migration) error {
	dbExists, err := ds.helper.CheckTableExists(tx, "apps")
	if err != nil {
		return err
	}
	if !dbExists {
		// set to highest and bail
		return migratex.SetVersion(ctx, tx, latestVersion(migrations), false)
	}

	// run any migrations needed to get to latest, if any
	return migratex.Up(ctx, tx, migrations)
}

// latest version will find the latest version from a list of migration
// names (not from the db)
func latestVersion(migs []migratex.Migration) int64 {
	var highest int64
	for _, mig := range migs {
		if mig.Version() > highest {
			highest = mig.Version()
		}
	}

	return highest
}

// clear is for tests only, be careful, it deletes all records.
func (ds *SQLStore) clear() error {
	return ds.Tx(func(tx *sqlx.Tx) error {
		query := tx.Rebind(`DELETE FROM routes`)
		_, err := tx.Exec(query)
		if err != nil {
			return err
		}

		query = tx.Rebind(`DELETE FROM calls`)
		_, err = tx.Exec(query)
		if err != nil {
			return err
		}

		query = tx.Rebind(`DELETE FROM apps`)
		_, err = tx.Exec(query)
		if err != nil {
			return err
		}

		query = tx.Rebind(`DELETE FROM triggers`)
		_, err = tx.Exec(query)
		if err != nil {
			return err
		}

		query = tx.Rebind(`DELETE FROM fns`)
		_, err = tx.Exec(query)
		if err != nil {
			return err
		}

		query = tx.Rebind(`DELETE FROM logs`)
		_, err = tx.Exec(query)
		return err
	})
}

func (ds *SQLStore) GetAppID(ctx context.Context, appName string) (string, error) {
	var app models.App
	query := ds.db.Rebind(ensureAppSelector)
	row := ds.db.QueryRowxContext(ctx, query, appName)

	err := row.StructScan(&app)
	if err == sql.ErrNoRows {
		return "", models.ErrAppsNotFound
	}
	if err != nil {
		return "", err
	}

	return app.ID, nil
}

func (ds *SQLStore) InsertApp(ctx context.Context, newApp *models.App) (*models.App, error) {
	app := newApp.Clone()
	app.CreatedAt = common.DateTime(time.Now())
	app.UpdatedAt = app.CreatedAt
	app.ID = id.New().String()

	if app.Config == nil {
		// keeps the JSON from being nil
		app.Config = map[string]string{}
	}

	query := ds.db.Rebind(`INSERT INTO apps (
		id,
		name,
		config,
		annotations,
		syslog_url,
		created_at,
		updated_at
	)
	VALUES (
		:id,
		:name,
		:config,
		:annotations,
		:syslog_url,
		:created_at,
		:updated_at
	);`)
	_, err := ds.db.NamedExecContext(ctx, query, app)
	if err != nil {
		if ds.helper.IsDuplicateKeyError(err) {
			return nil, models.ErrAppsAlreadyExists
		}
		return nil, err
	}

	return app, nil
}

func (ds *SQLStore) UpdateApp(ctx context.Context, newapp *models.App) (*models.App, error) {
	var app models.App

	err := ds.Tx(func(tx *sqlx.Tx) error {
		// NOTE: must query whole object since we're returning app, Update logic
		// must only modify modifiable fields (as seen here). need to fix brittle..

		query := tx.Rebind(appIDSelector)
		row := tx.QueryRowxContext(ctx, query, newapp.ID)

		err := row.StructScan(&app)
		if err == sql.ErrNoRows {
			return models.ErrAppsNotFound
		}
		if err != nil {
			return err
		}

		if newapp.Name != "" && app.Name != newapp.Name {
			return models.ErrAppsNameImmutable
		}
		app.Update(newapp)
		err = app.Validate()
		if err != nil {
			return err
		}

		query = tx.Rebind(`UPDATE apps SET config=:config, annotations=:annotations, syslog_url=:syslog_url, updated_at=:updated_at WHERE name=:name`)
		res, err := tx.NamedExecContext(ctx, query, app)
		if err != nil {
			return err
		}

		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			// inside of the transaction, we are querying for the app, so we know that it exists
			return nil
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &app, nil
}

func (ds *SQLStore) RemoveApp(ctx context.Context, appID string) error {
	return ds.Tx(func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM apps WHERE id=?`), appID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return models.ErrAppsNotFound
		}

		deletes := []string{
			`DELETE FROM logs WHERE app_id=?`,
			`DELETE FROM calls WHERE app_id=?`,
			`DELETE FROM routes WHERE app_id=?`,
			`DELETE FROM fns WHERE app_id=?`,
			`DELETE FROM triggers WHERE app_id=?`,
		}
		for _, stmt := range deletes {
			_, err := tx.ExecContext(ctx, tx.Rebind(stmt), appID)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (ds *SQLStore) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	var app models.App
	query := ds.db.Rebind(appIDSelector)
	row := ds.db.QueryRowxContext(ctx, query, appID)

	err := row.StructScan(&app)
	if err == sql.ErrNoRows {
		return nil, models.ErrAppsNotFound
	}
	if err != nil {
		return nil, err
	}
	return &app, err
}

// GetApps retrieves an array of apps according to a specific filter.
func (ds *SQLStore) GetApps(ctx context.Context, filter *models.AppFilter) (*models.AppList, error) {
	res := &models.AppList{Items: []*models.App{}}

	query, args, err := buildFilterAppQuery(filter)
	if err != nil {
		return nil, err
	}
	query = ds.db.Rebind(fmt.Sprintf("SELECT DISTINCT id, name, config, annotations, syslog_url, created_at, updated_at FROM apps %s", query))
	rows, err := ds.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var app models.App
		err := rows.StructScan(&app)
		if err != nil {
			if err == sql.ErrNoRows {
				return res, nil
			}
			return res, err
		}
		res.Items = append(res.Items, &app)
	}

	if len(res.Items) > 0 && len(res.Items) == filter.PerPage {
		last := []byte(res.Items[len(res.Items)-1].Name)
		res.NextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	if err := rows.Err(); err != nil {
		return res, err
	}
	return res, nil
}

func (ds *SQLStore) InsertRoute(ctx context.Context, newRoute *models.Route) (*models.Route, error) {
	route := newRoute.Clone()
	route.CreatedAt = common.DateTime(time.Now())
	route.UpdatedAt = route.CreatedAt

	err := ds.Tx(func(tx *sqlx.Tx) error {
		query := tx.Rebind(`SELECT 1 FROM apps WHERE id=?`)
		r := tx.QueryRowContext(ctx, query, route.AppID)
		if err := r.Scan(new(int)); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			}
		}
		query = tx.Rebind(`SELECT 1 FROM routes WHERE app_id=? AND path=?`)
		same, err := tx.QueryContext(ctx, query, route.AppID, route.Path)
		if err != nil {
			return err
		}
		defer same.Close()
		if same.Next() {
			return models.ErrRoutesAlreadyExists
		}

		query = tx.Rebind(`INSERT INTO routes (
			app_id,
			path,
			image,
			format,
			memory,
			cpus,
			type,
			timeout,
			idle_timeout,
			tmpfs_size,
			headers,
			config,
			annotations,
			created_at,
			updated_at
		)
		VALUES (
			:app_id,
			:path,
			:image,
			:format,
			:memory,
			:cpus,
			:type,
			:timeout,
			:idle_timeout,
			:tmpfs_size,
			:headers,
			:config,
			:annotations,
			:created_at,
			:updated_at
		);`)

		_, err = tx.NamedExecContext(ctx, query, route)

		return err
	})

	return route, err
}

func (ds *SQLStore) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	var route models.Route
	err := ds.Tx(func(tx *sqlx.Tx) error {
		query := tx.Rebind(fmt.Sprintf("%s WHERE app_id=? AND path=?", routeSelector))
		row := tx.QueryRowxContext(ctx, query, newroute.AppID, newroute.Path)

		err := row.StructScan(&route)
		if err == sql.ErrNoRows {
			return models.ErrRoutesNotFound
		} else if err != nil {
			return err
		}

		route.Update(newroute)
		err = route.Validate()
		if err != nil {
			return err
		}

		query = tx.Rebind(`UPDATE routes SET
			image = :image,
			format = :format,
			memory = :memory,
			cpus = :cpus,
			type = :type,
			timeout = :timeout,
			idle_timeout = :idle_timeout,
			tmpfs_size = :tmpfs_size,
			headers = :headers,
			config = :config,
			annotations = :annotations,
			updated_at = :updated_at
		WHERE app_id=:app_id AND path=:path;`)

		res, err := tx.NamedExecContext(ctx, query, &route)
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

func (ds *SQLStore) RemoveRoute(ctx context.Context, appID string, routePath string) error {
	query := ds.db.Rebind(`DELETE FROM routes WHERE path = ? AND app_id = ?`)
	res, err := ds.db.ExecContext(ctx, query, routePath, appID)
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

func (ds *SQLStore) GetRoute(ctx context.Context, appID, routePath string) (*models.Route, error) {
	rSelectCondition := "%s WHERE app_id=? AND path=?"
	query := ds.db.Rebind(fmt.Sprintf(rSelectCondition, routeSelector))
	row := ds.db.QueryRowxContext(ctx, query, appID, routePath)

	var route models.Route
	err := row.StructScan(&route)
	if err == sql.ErrNoRows {
		return nil, models.ErrRoutesNotFound
	} else if err != nil {
		return nil, err
	}
	return &route, nil
}

// GetRoutesByApp retrieves a route with a specific app name.
func (ds *SQLStore) GetRoutesByApp(ctx context.Context, appID string, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	if filter == nil {
		filter = new(models.RouteFilter)
	}

	filter.AppID = appID
	filterQuery, args := buildFilterRouteQuery(filter)

	query := fmt.Sprintf("%s %s", routeSelector, filterQuery)
	query = ds.db.Rebind(query)
	rows, err := ds.db.QueryxContext(ctx, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return res, nil // no error for empty list
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var route models.Route
		err := rows.StructScan(&route)
		if err != nil {
			continue
		}
		res = append(res, &route)
	}

	if err := rows.Err(); err != nil {
		if err == sql.ErrNoRows {
			return res, nil // no error for empty list
		}
	}

	return res, nil
}

func (ds *SQLStore) InsertFn(ctx context.Context, newFn *models.Fn) (*models.Fn, error) {
	fn := newFn.Clone()
	fn.ID = id.New().String()
	fn.CreatedAt = common.DateTime(time.Now())
	fn.UpdatedAt = fn.CreatedAt

	err := newFn.Validate()
	if err != nil {
		return nil, err
	}

	err = ds.Tx(func(tx *sqlx.Tx) error {

		query := tx.Rebind(`SELECT 1 FROM apps WHERE id=?`)
		r := tx.QueryRowContext(ctx, query, fn.AppID)
		if err := r.Scan(new(int)); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			}
		}

		query = tx.Rebind(`INSERT INTO fns (
				id,
				name,
				app_id,
				image,
				format,
				memory,
				timeout,
				idle_timeout,
				config,
				annotations,
				created_at,
				updated_at
			)
			VALUES (
				:id,
				:name,
				:app_id,
				:image,
				:format,
				:memory,
				:timeout,
				:idle_timeout,
				:config,
				:annotations,
				:created_at,
				:updated_at
			);`)

		_, err = tx.NamedExecContext(ctx, query, fn)
		return err
	})

	if err != nil {
		if ds.helper.IsDuplicateKeyError(err) {
			return nil, models.ErrFnsExists
		}
		return nil, err
	}
	return fn, nil
}

func (ds *SQLStore) UpdateFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	err := ds.Tx(func(tx *sqlx.Tx) error {

		var dst models.Fn
		query := tx.Rebind(fnIDSelector)
		row := tx.QueryRowxContext(ctx, query, fn.ID)
		err := row.StructScan(&dst)

		if err == sql.ErrNoRows {
			return models.ErrFnsNotFound
		} else if err != nil {
			return err
		}

		dst.Update(fn)
		err = dst.Validate()
		if err != nil {
			return err
		}
		fn = &dst // set for query & to return

		query = tx.Rebind(`UPDATE fns SET
				name = :name,
				image = :image,
				format = :format,
				memory = :memory,
				timeout = :timeout,
				idle_timeout = :idle_timeout,
				config = :config,
				annotations = :annotations,
				updated_at = :updated_at
			    WHERE id=:id;`)

		_, err = tx.NamedExecContext(ctx, query, fn)
		return err
	})

	if err != nil {
		return nil, err
	}
	return fn, nil
}

func (ds *SQLStore) GetFns(ctx context.Context, filter *models.FnFilter) (*models.FnList, error) {
	res := &models.FnList{Items: []*models.Fn{}}
	if filter == nil {
		filter = new(models.FnFilter)
	}

	filterQuery, args, err := buildFilterFnQuery(filter)
	if err != nil {
		return res, err
	}

	query := fmt.Sprintf("%s %s", fnSelector, filterQuery)
	query = ds.db.Rebind(query)
	rows, err := ds.db.QueryxContext(ctx, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return res, nil // no error for empty list
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var fn models.Fn
		err := rows.StructScan(&fn)
		if err != nil {
			continue
		}
		res.Items = append(res.Items, &fn)
	}

	if len(res.Items) > 0 && len(res.Items) == filter.PerPage {
		last := []byte(res.Items[len(res.Items)-1].Name)
		res.NextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	if err := rows.Err(); err != nil {
		if err == sql.ErrNoRows {
			return res, nil // no error for empty list
		}
	}
	return res, nil
}

func (ds *SQLStore) GetFnByID(ctx context.Context, fnID string) (*models.Fn, error) {
	query := ds.db.Rebind(fmt.Sprintf("%s WHERE id=?", fnSelector))
	row := ds.db.QueryRowxContext(ctx, query, fnID)

	var fn models.Fn
	err := row.StructScan(&fn)
	if err == sql.ErrNoRows {
		return nil, models.ErrFnsNotFound
	} else if err != nil {
		return nil, err
	}
	return &fn, nil
}

func (ds *SQLStore) RemoveFn(ctx context.Context, fnID string) error {

	return ds.Tx(func(tx *sqlx.Tx) error {

		query := tx.Rebind(fmt.Sprintf("%s WHERE id=?", fnSelector))
		row := tx.QueryRowxContext(ctx, query, fnID)

		var fn models.Fn
		err := row.StructScan(&fn)
		if err == sql.ErrNoRows {
			return models.ErrFnsNotFound
		}

		query = tx.Rebind(`DELETE FROM triggers WHERE fn_id=?`)
		_, err = tx.ExecContext(ctx, query, fnID)

		if err != nil {
			return err
		}

		query = tx.Rebind(`DELETE FROM fns WHERE id=?`)
		_, err = tx.ExecContext(ctx, query, fnID)

		return err
	})

}

func (ds *SQLStore) Tx(f func(*sqlx.Tx) error) error {
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

func (ds *SQLStore) InsertCall(ctx context.Context, call *models.Call) error {
	query := ds.db.Rebind(`INSERT INTO calls (
		id,
		created_at,
		started_at,
		completed_at,
		status,
		app_id,
		fn_id,
		path,
		stats,
		error
	)
	VALUES (
		:id,
		:created_at,
		:started_at,
		:completed_at,
		:status,
		:app_id,
		:fn_id,
		:path,
		:stats,
		:error
	);`)

	_, err := ds.db.NamedExecContext(ctx, query, call)
	return err
}

func (ds *SQLStore) GetCall(ctx context.Context, callID string) (*models.Call, error) {
	query := fmt.Sprintf(`%s WHERE id=?`, callSelector)
	query = ds.db.Rebind(query)
	row := ds.db.QueryRowxContext(ctx, query, callID)

	var call models.Call
	err := row.StructScan(&call)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrCallNotFound
		}
		return nil, err
	}
	return &call, nil
}

func (ds *SQLStore) GetCalls(ctx context.Context, filter *models.CallFilter) (*models.CallList, error) {
	res := &models.CallList{Items: []*models.Call{}}
	query, args := buildFilterCallQuery(filter)
	query = fmt.Sprintf("%s %s", callSelector, query)
	query = ds.db.Rebind(query)
	rows, err := ds.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var call models.Call
		err := rows.StructScan(&call)
		if err != nil {
			continue
		}
		res.Items = append(res.Items, &call)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *SQLStore) InsertLog(ctx context.Context, appID, fnID, callID string, logR io.Reader) error {
	// coerce this into a string for sql
	var log string
	if stringer, ok := logR.(fmt.Stringer); ok {
		log = stringer.String()
	} else {
		// TODO we could optimize for Size / buffer pool, but atm we aren't hitting
		// this code path anyway (a fallback)
		var b bytes.Buffer
		io.Copy(&b, logR)
		log = b.String()
	}

	query := ds.db.Rebind(`INSERT INTO logs (id, app_id, fn_id, log) VALUES (?, ?, ?, ?);`)
	_, err := ds.db.ExecContext(ctx, query, callID, appID, fnID, log)

	return err
}

func (ds *SQLStore) GetLog(ctx context.Context, callID string) (io.Reader, error) {
	query := ds.db.Rebind(`SELECT log FROM logs WHERE id=?`)
	row := ds.db.QueryRowContext(ctx, query, callID)

	var log string
	err := row.Scan(&log)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrCallLogNotFound
		}
		return nil, err
	}

	return strings.NewReader(log), nil
}

func buildFilterRouteQuery(filter *models.RouteFilter) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}
	var b bytes.Buffer
	var args []interface{}

	args = where(&b, args, "app_id=? ", filter.AppID)
	args = where(&b, args, "image=?", filter.Image)
	args = where(&b, args, "path>?", filter.Cursor)
	// where("path LIKE ?%", filter.PathPrefix) TODO needs escaping

	fmt.Fprintf(&b, ` ORDER BY path ASC`) // TODO assert this is indexed
	fmt.Fprintf(&b, ` LIMIT ?`)
	args = append(args, filter.PerPage)

	return b.String(), args
}

func buildFilterAppQuery(filter *models.AppFilter) (string, []interface{}, error) {
	var args []interface{}
	if filter == nil {
		return "", args, nil
	}

	var b bytes.Buffer

	if filter.Cursor != "" {
		s, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return "", args, err
		}
		args = where(&b, args, "name>?", string(s))
	}
	if filter.Name != "" {
		args = where(&b, args, "name=?", filter.Name)
	}

	fmt.Fprintf(&b, ` ORDER BY name ASC`) // TODO assert this is indexed
	fmt.Fprintf(&b, ` LIMIT ?`)
	args = append(args, filter.PerPage)
	return b.String(), args, nil
}

func buildFilterCallQuery(filter *models.CallFilter) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}
	var b bytes.Buffer
	var args []interface{}

	args = where(&b, args, "id<?", filter.Cursor)
	if !time.Time(filter.ToTime).IsZero() {
		args = where(&b, args, "created_at<?", filter.ToTime.String())
	}
	if !time.Time(filter.FromTime).IsZero() {
		args = where(&b, args, "created_at>?", filter.FromTime.String())
	}
	args = where(&b, args, "fn_id=?", filter.FnID)
	args = where(&b, args, "path=?", filter.Path)

	fmt.Fprintf(&b, ` ORDER BY id DESC`) // TODO assert this is indexed
	fmt.Fprintf(&b, ` LIMIT ?`)
	args = append(args, filter.PerPage)

	return b.String(), args
}

func buildFilterFnQuery(filter *models.FnFilter) (string, []interface{}, error) {
	if filter == nil {
		return "", nil, nil
	}
	var b bytes.Buffer
	var args []interface{}

	// where(fmt.Sprintf("image LIKE '%s%%'"), filter.Image) // TODO needs escaping, prob we want prefix query to ignore tags
	args = where(&b, args, "app_id=? ", filter.AppID)

	if filter.Cursor != "" {
		s, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return "", args, err
		}
		args = where(&b, args, "name>?", string(s))
	}

	fmt.Fprintf(&b, ` ORDER BY name ASC`)
	if filter.PerPage > 0 {
		fmt.Fprintf(&b, ` LIMIT ?`)
		args = append(args, filter.PerPage)
	}
	return b.String(), args, nil
}

func where(b *bytes.Buffer, args []interface{}, colOp string, val interface{}) []interface{} {
	if val == nil {
		return args
	}
	switch v := val.(type) {
	case string:
		if v == "" {
			return args
		}
	case []string:
		if len(v) == 0 {
			return args
		}
	}
	args = append(args, val)
	if len(args) == 1 {
		fmt.Fprintf(b, `WHERE %s`, colOp)
	} else {
		fmt.Fprintf(b, ` AND %s`, colOp)
	}
	return args
}

func (ds *SQLStore) InsertTrigger(ctx context.Context, newTrigger *models.Trigger) (*models.Trigger, error) {

	trigger := newTrigger.Clone()

	trigger.CreatedAt = common.DateTime(time.Now())
	trigger.UpdatedAt = trigger.CreatedAt
	trigger.ID = id.New().String()

	err := trigger.Validate()
	if err != nil {
		return nil, err
	}

	err = ds.Tx(func(tx *sqlx.Tx) error {
		query := tx.Rebind(`SELECT 1 FROM apps WHERE id=?`)
		r := tx.QueryRowContext(ctx, query, trigger.AppID)
		if err := r.Scan(new(int)); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrAppsNotFound
			} else if err != nil {
				return err
			}
		}

		query = tx.Rebind(`SELECT app_id FROM fns WHERE id=?`)
		r = tx.QueryRowContext(ctx, query, trigger.FnID)
		var app_id string
		if err := r.Scan(&app_id); err != nil {
			if err == sql.ErrNoRows {
				return models.ErrFnsNotFound
			} else if err != nil {
				return err
			}
		}
		if app_id != trigger.AppID {
			return models.ErrTriggerFnIDNotSameApp
		}

		query = tx.Rebind(`SELECT 1 FROM triggers WHERE app_id=? AND type=? and source=?`)
		r = tx.QueryRowContext(ctx, query, trigger.AppID, trigger.Type, trigger.Source)
		err := r.Scan(new(int))
		if err == nil {
			return models.ErrTriggerSourceExists
		} else if err != sql.ErrNoRows {
			return err
		}

		query = tx.Rebind(`INSERT INTO triggers (
			id,
			name,
		  	app_id,
			fn_id,
			created_at,
			updated_at,
			type,
		  	source,
		  	annotations
		)
		VALUES (
			:id,
			:name,
			:app_id,
			:fn_id,
			:created_at,
			:updated_at,
			:type,
			:source,
			:annotations
		);`)

		_, err = tx.NamedExecContext(ctx, query, trigger)
		return err
	})

	if err != nil {
		if ds.helper.IsDuplicateKeyError(err) {
			return nil, models.ErrTriggerExists
		}
		return nil, err
	}

	return trigger, err
}

func (ds *SQLStore) UpdateTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {
	err := ds.Tx(func(tx *sqlx.Tx) error {

		var dst models.Trigger
		query := tx.Rebind(triggerIDSelector)
		row := tx.QueryRowxContext(ctx, query, trigger.ID)
		err := row.StructScan(&dst)

		if err != nil && err != sql.ErrNoRows {
			return err
		} else if err == sql.ErrNoRows {
			return models.ErrTriggerNotFound
		}

		dst.Update(trigger)
		err = dst.Validate()
		if err != nil {
			return err
		}
		trigger = &dst // set for query & to return

		query = tx.Rebind(`UPDATE triggers SET
			name = :name,
			fn_id = :fn_id,
			updated_at = :updated_at,
			source = :source,
			annotations = :annotations
			WHERE id = :id;`)
		_, err = tx.NamedExecContext(ctx, query, trigger)
		return err
	})

	if err != nil {
		return nil, err
	}
	return trigger, nil
}

func (ds *SQLStore) GetTrigger(ctx context.Context, appId, fnId, triggerName string) (*models.Trigger, error) {
	var trigger models.Trigger
	query := ds.db.Rebind(fmt.Sprintf("%s WHERE name=? AND app_id=? AND fn_id=?", fnSelector))
	row := ds.db.QueryRowxContext(ctx, query, triggerName, appId, fnId)

	err := row.StructScan(&trigger)
	if err == sql.ErrNoRows {
		return nil, models.ErrTriggerNotFound
	}
	if err != nil {
		return nil, err
	}

	return &trigger, nil
}

func (ds *SQLStore) RemoveTrigger(ctx context.Context, triggerId string) error {
	query := ds.db.Rebind(`DELETE FROM triggers WHERE id = ?;`)
	res, err := ds.db.ExecContext(ctx, query, triggerId)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return models.ErrTriggerNotFound
	}

	return nil
}

func (ds *SQLStore) GetTriggerByID(ctx context.Context, triggerID string) (*models.Trigger, error) {
	var trigger models.Trigger
	query := ds.db.Rebind(triggerIDSelector)
	row := ds.db.QueryRowxContext(ctx, query, triggerID)

	err := row.StructScan(&trigger)
	if err == sql.ErrNoRows {
		return nil, models.ErrTriggerNotFound
	} else if err != nil {
		return nil, err
	}

	return &trigger, nil
}

func buildFilterTriggerQuery(filter *models.TriggerFilter) (string, []interface{}, error) {
	var b bytes.Buffer
	var args []interface{}

	fmt.Fprintf(&b, `app_id = ?`)
	args = append(args, filter.AppID)

	if filter.FnID != "" {
		fmt.Fprintf(&b, ` AND fn_id = ?`)
		args = append(args, filter.FnID)
	}

	if filter.Name != "" {
		fmt.Fprintf(&b, ` AND name = ?`)
		args = append(args, filter.Name)
	}

	if filter.Cursor != "" {
		s, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return "", nil, err
		}

		fmt.Fprintf(&b, ` AND name > ?`)
		args = append(args, string(s))
	}

	fmt.Fprintf(&b, ` ORDER BY name ASC`)

	if filter.PerPage > 0 {
		fmt.Fprintf(&b, ` LIMIT ?`)
		args = append(args, filter.PerPage)
	}

	return b.String(), args, nil
}

func (ds *SQLStore) GetTriggers(ctx context.Context, filter *models.TriggerFilter) (*models.TriggerList, error) {
	res := &models.TriggerList{Items: []*models.Trigger{}}
	if filter == nil {
		filter = new(models.TriggerFilter)
	}

	filterQuery, args, err := buildFilterTriggerQuery(filter)
	if err != nil {
		return res, err
	}

	query := fmt.Sprintf("%s WHERE %s", triggerSelector, filterQuery)
	query = ds.db.Rebind(query)
	rows, err := ds.db.QueryxContext(ctx, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return res, nil // no error for empty list
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var trigger models.Trigger
		err := rows.StructScan(&trigger)
		if err != nil {
			continue
		}
		res.Items = append(res.Items, &trigger)
	}

	if len(res.Items) > 0 && len(res.Items) == filter.PerPage {
		last := []byte(res.Items[len(res.Items)-1].Name)
		res.NextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	if err := rows.Err(); err != nil {
		if err == sql.ErrNoRows {
			return res, nil // no error for empty list
		}
		return nil, err
	}
	return res, nil
}

func (ds *SQLStore) GetTriggerBySource(ctx context.Context, appId string, triggerType, source string) (*models.Trigger, error) {
	var trigger models.Trigger

	query := ds.db.Rebind(triggerIDSourceSelector)
	row := ds.db.QueryRowxContext(ctx, query, appId, triggerType, source)

	err := row.StructScan(&trigger)
	if err == sql.ErrNoRows {
		return nil, models.ErrTriggerNotFound
	} else if err != nil {
		return nil, err
	}
	return &trigger, nil
}

// Close closes the database, releasing any open resources.
func (ds *SQLStore) Close() error {
	return ds.db.Close()
}

func init() {
	datastore.Register(sqlDsProvider(0))
	logs.Register(sqlLogsProvider(0))
}
