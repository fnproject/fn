package datastoreutil

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"gitlab-odx.oracle.com/odx/functions/api/models"
)

// TODO scrap for sqlx

type RowScanner interface {
	Scan(dest ...interface{}) error
}

func ScanLog(scanner RowScanner, log *models.FnCallLog) error {
	return scanner.Scan(
		&log.CallID,
		&log.Log,
	)
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
