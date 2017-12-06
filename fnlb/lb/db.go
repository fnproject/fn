package lb

import (
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

func NewDB(conf Config) (DBStore, error) {
	if conf.DBurl == "k8s" {
		return NewK8sStore(conf)
	}
	db, err := db(conf.DBurl)
	if err != nil {
		return nil, err
	}

	return db, err
}

// TODO put this somewhere better
type DBStore interface {
	Add(string) error
	Delete(string) error
	List() ([]string, error)
}

// implements DBStore
type sqlStore struct {
	db *sqlx.DB

	// TODO we should prepare all of the statements, rebind them
	// and store them all here.
}

// New will open the db specified by url, create any tables necessary
// and return a models.Datastore safe for concurrent usage.
func db(uri string) (DBStore, error) {
	url, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	driver := url.Scheme
	// driver must be one of these for sqlx to work, double check:
	switch driver {
	case "postgres", "pgx", "mysql", "sqlite3", "oci8", "ora", "goracle":
	default:
		return nil, errors.New("invalid db driver, refer to the code")
	}

	if driver == "sqlite3" {
		// make all the dirs so we can make the file..
		dir := filepath.Dir(url.Path)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}

	uri = url.String()
	if driver != "postgres" {
		// postgres seems to need this as a prefix in lib/pq, everyone else wants it stripped of scheme
		uri = strings.TrimPrefix(url.String(), url.Scheme+"://")
	}

	sqldb, err := sql.Open(driver, uri)
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't open db")
		return nil, err
	}

	db := sqlx.NewDb(sqldb, driver)
	// force a connection and test that it worked
	err = db.Ping()
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't ping db")
		return nil, err
	}

	maxIdleConns := 30 // c.MaxIdleConnections
	db.SetMaxIdleConns(maxIdleConns)
	logrus.WithFields(logrus.Fields{"max_idle_connections": maxIdleConns, "datastore": driver}).Info("datastore dialed")

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS lb_nodes (
        address text NOT NULL PRIMARY KEY
    );`)
	if err != nil {
		return nil, err
	}

	return &sqlStore{db: db}, nil
}

func (s *sqlStore) Add(node string) error {
	query := s.db.Rebind("INSERT INTO lb_nodes (address) VALUES (?);")
	_, err := s.db.Exec(query, node)
	if err != nil {
		// if it already exists, just filter that error out
		switch err := err.(type) {
		case *mysql.MySQLError:
			if err.Number == 1062 {
				return nil
			}
		case *pq.Error:
			if err.Code == "23505" {
				return nil
			}
		case sqlite3.Error:
			if err.ExtendedCode == sqlite3.ErrConstraintUnique || err.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
				return nil
			}
		}
	}
	return err
}

func (s *sqlStore) Delete(node string) error {
	query := s.db.Rebind(`DELETE FROM lb_nodes WHERE address=?`)
	_, err := s.db.Exec(query, node)
	// TODO we can filter if it didn't exist, too...
	return err
}

func (s *sqlStore) List() ([]string, error) {
	query := s.db.Rebind(`SELECT DISTINCT address FROM lb_nodes`)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}

	var nodes []string
	for rows.Next() {
		var node string
		err := rows.Scan(&node)
		if err == nil {
			nodes = append(nodes, node)
		}
	}

	err = rows.Err()
	if err == sql.ErrNoRows {
		err = nil // don't care...
	}

	return nodes, err
}
