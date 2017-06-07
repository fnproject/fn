package datastoreutil

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"strings"

	"gitlab-odx.oracle.com/odx/functions/api/models"
)

type RowScanner interface {
	Scan(dest ...interface{}) error
}

func ScanRoute(scanner RowScanner, route *models.Route) error {
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

func ScanApp(scanner RowScanner, app *models.App) error {
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

func BuildFilterRouteQuery(filter *models.RouteFilter, whereStm, andStm string) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}
	var b bytes.Buffer
	var args []interface{}

	where := func(colOp, val string) {
		if val != "" {
			args = append(args, val)
			if len(args) == 1 {
				fmt.Fprintf(&b, whereStm, colOp)
			} else {
				//TODO: maybe better way to detect/driver SQL dialect-specific things
				if strings.Contains(whereStm, "$") {
					// PgSQL specific
					fmt.Fprintf(&b, andStm, colOp, len(args))
				} else {
					// MySQL specific
					fmt.Fprintf(&b, andStm, colOp)
				}
			}
		}
	}

	where("path =", filter.Path)
	where("app_name =", filter.AppName)
	where("image =", filter.Image)

	return b.String(), args
}

func BuildFilterAppQuery(filter *models.AppFilter, whereStm string) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}

	if filter.Name != "" {
		return whereStm, []interface{}{filter.Name}
	}

	return "", nil
}

func BuildFilterCallQuery(filter *models.CallFilter, whereStm, andStm string) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}
	var b bytes.Buffer
	var args []interface{}

	where := func(colOp, val string) {
		if val != "" {
			args = append(args, val)
			if len(args) == 1 {
				fmt.Fprintf(&b, whereStm, colOp)
			} else {
				fmt.Fprintf(&b, andStm, colOp)
			}
		}
	}

	where("path =", filter.Path)
	where("app_name =", filter.AppName)

	return b.String(), args
}

func ScanCall(scanner RowScanner, call *models.FnCall) error {
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

func SQLGetCall(db *sql.DB, callSelector, callID, whereStm string) (*models.FnCall, error) {
	var call models.FnCall
	row := db.QueryRow(fmt.Sprintf(whereStm, callSelector), callID)
	err := ScanCall(row, &call)
	if err != nil {
		return nil, err
	}
	return &call, nil
}

func SQLGetCalls(db *sql.DB, cSelector string, filter *models.CallFilter, whereStm, andStm string) (models.FnCalls, error) {
	res := models.FnCalls{}
	filterQuery, args := BuildFilterCallQuery(filter, whereStm, andStm)
	rows, err := db.Query(fmt.Sprintf("%s %s", cSelector, filterQuery), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var call models.FnCall
		err := ScanCall(rows, &call)
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

func SQLGetApp(db *sql.DB, queryStr string, queryArgs ...interface{}) (*models.App, error) {
	row := db.QueryRow(queryStr, queryArgs...)

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

func SQLGetApps(db *sql.DB, filter *models.AppFilter, whereStm, selectStm string) ([]*models.App, error) {
	res := []*models.App{}
	filterQuery, args := BuildFilterAppQuery(filter, whereStm)
	rows, err := db.Query(fmt.Sprintf(selectStm, filterQuery), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var app models.App
		err := ScanApp(rows, &app)

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

func NewDatastore(dataSourceName, dialect string, tables []string) (*sql.DB, error) {
	db, err := sql.Open(dialect, dataSourceName)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	maxIdleConns := 30 // c.MaxIdleConnections
	db.SetMaxIdleConns(maxIdleConns)
	logrus.WithFields(logrus.Fields{"max_idle_connections": maxIdleConns}).Info(
		fmt.Sprintf("%v datastore dialed", dialect))

	for _, v := range tables {
		_, err = db.Exec(v)
		if err != nil {
			return nil, err
		}
	}

	return db, nil
}

func SQLGetRoutes(db *sql.DB, filter *models.RouteFilter, rSelect string, whereStm, andStm string) ([]*models.Route, error) {
	res := []*models.Route{}
	filterQuery, args := BuildFilterRouteQuery(filter, whereStm, andStm)
	rows, err := db.Query(fmt.Sprintf("%s %s", rSelect, filterQuery), args...)
	// todo: check for no rows so we don't respond with a sql 500 err
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var route models.Route
		err := ScanRoute(rows, &route)
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

func SQLGetRoutesByApp(db *sql.DB, appName string, filter *models.RouteFilter, rSelect, defaultFilterQuery, whereStm, andStm string) ([]*models.Route, error) {
	res := []*models.Route{}
	var filterQuery string
	var args []interface{}
	if filter == nil {
		filterQuery = defaultFilterQuery
		args = []interface{}{appName}
	} else {
		filter.AppName = appName
		filterQuery, args = BuildFilterRouteQuery(filter, whereStm, andStm)
	}
	rows, err := db.Query(fmt.Sprintf("%s %s", rSelect, filterQuery), args...)
	// todo: check for no rows so we don't respond with a sql 500 err
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var route models.Route
		err := ScanRoute(rows, &route)
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

func SQLGetRoute(db *sql.DB, appName, routePath, rSelectCondition, routeSelector string) (*models.Route, error) {
	var route models.Route

	row := db.QueryRow(fmt.Sprintf(rSelectCondition, routeSelector), appName, routePath)
	err := ScanRoute(row, &route)

	if err == sql.ErrNoRows {
		return nil, models.ErrRoutesNotFound
	} else if err != nil {
		return nil, err
	}
	return &route, nil
}

func SQLRemoveRoute(db *sql.DB, appName, routePath, deleteStm string) error {
	res, err := db.Exec(deleteStm, routePath, appName)

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
