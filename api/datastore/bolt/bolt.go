package bolt

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"context"

	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/iron-io/functions/api/models"
	"regexp"
	"strings"
)

type BoltDatastore struct {
	routesBucket []byte
	appsBucket   []byte
	logsBucket   []byte
	extrasBucket []byte
	db           *bolt.DB
	log          logrus.FieldLogger
}

func New(url *url.URL) (models.Datastore, error) {
	dir := filepath.Dir(url.Path)
	log := logrus.WithFields(logrus.Fields{"db": url.Scheme, "dir": dir})
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		log.WithError(err).Errorln("Could not create data directory for db")
		return nil, err
	}
	log.Infoln("Creating bolt db at ", url.Path)
	db, err := bolt.Open(url.Path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.WithError(err).Errorln("Error on bolt.Open")
		return nil, err
	}
	// I don't think we need a prefix here do we? Made it blank. If we do, we should call the query param "prefix" instead of bucket.
	bucketPrefix := ""
	if url.Query()["bucket"] != nil {
		bucketPrefix = url.Query()["bucket"][0]
	}
	routesBucketName := []byte(bucketPrefix + "routes")
	appsBucketName := []byte(bucketPrefix + "apps")
	logsBucketName := []byte(bucketPrefix + "logs")
	extrasBucketName := []byte(bucketPrefix + "extras") // todo: think of a better name
	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{routesBucketName, appsBucketName, logsBucketName, extrasBucketName} {
			_, err := tx.CreateBucketIfNotExists(name)
			if err != nil {
				log.WithError(err).WithFields(logrus.Fields{"name": name}).Error("create bucket")
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Errorln("Error creating bolt buckets")
		return nil, err
	}

	ds := &BoltDatastore{
		routesBucket: routesBucketName,
		appsBucket:   appsBucketName,
		logsBucket:   logsBucketName,
		extrasBucket: extrasBucketName,
		db:           db,
		log:          log,
	}
	log.WithFields(logrus.Fields{"prefix": bucketPrefix, "file": url.Path}).Info("BoltDB initialized")

	return ds, nil
}

func (ds *BoltDatastore) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}

	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	appname := []byte(app.Name)

	err := ds.db.Update(func(tx *bolt.Tx) error {
		bIm := tx.Bucket(ds.appsBucket)

		v := bIm.Get(appname)
		if v != nil {
			return models.ErrAppsAlreadyExists
		}

		buf, err := json.Marshal(app)
		if err != nil {
			return err
		}

		err = bIm.Put(appname, buf)
		if err != nil {
			return err
		}
		bjParent := tx.Bucket(ds.routesBucket)
		_, err = bjParent.CreateBucketIfNotExists([]byte(app.Name))
		if err != nil {
			return err
		}
		return nil
	})

	return app, err
}

func (ds *BoltDatastore) UpdateApp(ctx context.Context, newapp *models.App) (*models.App, error) {
	if newapp == nil {
		return nil, models.ErrDatastoreEmptyApp
	}

	if newapp.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	var app *models.App
	appname := []byte(newapp.Name)

	err := ds.db.Update(func(tx *bolt.Tx) error {
		bIm := tx.Bucket(ds.appsBucket)

		v := bIm.Get(appname)
		if v == nil {
			return models.ErrAppsNotFound
		}

		err := json.Unmarshal(v, &app)
		if err != nil {
			return err
		}

		// Update app fields
		if newapp.Config != nil {
			if app.Config == nil {
				app.Config = map[string]string{}
			}
			for k, v := range newapp.Config {
				app.Config[k] = v
			}
		}

		buf, err := json.Marshal(app)
		if err != nil {
			return err
		}

		err = bIm.Put(appname, buf)
		if err != nil {
			return err
		}
		bjParent := tx.Bucket(ds.routesBucket)
		_, err = bjParent.CreateBucketIfNotExists([]byte(app.Name))
		if err != nil {
			return err
		}
		return nil
	})

	return app, err
}

func (ds *BoltDatastore) RemoveApp(ctx context.Context, appName string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}

	err := ds.db.Update(func(tx *bolt.Tx) error {
		bIm := tx.Bucket(ds.appsBucket)
		err := bIm.Delete([]byte(appName))
		if err != nil {
			return err
		}
		bjParent := tx.Bucket(ds.routesBucket)
		err = bjParent.DeleteBucket([]byte(appName))
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

func (ds *BoltDatastore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}
	err := ds.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(ds.appsBucket)
		err2 := b.ForEach(func(key, v []byte) error {
			app := &models.App{}
			err := json.Unmarshal(v, app)
			if err != nil {
				return err
			}
			if applyAppFilter(app, filter) {
				res = append(res, app)
			}
			return nil
		})
		if err2 != nil {
			logrus.WithError(err2).Errorln("Couldn't get apps!")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *BoltDatastore) GetApp(ctx context.Context, name string) (*models.App, error) {
	if name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	var res *models.App
	err := ds.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(ds.appsBucket)
		v := b.Get([]byte(name))
		if v != nil {
			app := &models.App{}
			err := json.Unmarshal(v, app)
			if err != nil {
				return err
			}
			res = app
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *BoltDatastore) getRouteBucketForApp(tx *bolt.Tx, appName string) (*bolt.Bucket, error) {
	var err error
	// todo: should this be reversed?  Make a bucket for each app that contains sub buckets for routes, etc
	bp := tx.Bucket(ds.routesBucket)
	b := bp.Bucket([]byte(appName))
	if b == nil {
		b, err = bp.CreateBucket([]byte(appName))
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (ds *BoltDatastore) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyApp
	}

	if route.AppName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if route.Path == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	routePath := []byte(route.Path)

	err := ds.db.Update(func(tx *bolt.Tx) error {
		b, err := ds.getRouteBucketForApp(tx, route.AppName)
		if err != nil {
			return err
		}

		v := b.Get(routePath)
		if v != nil {
			return models.ErrRoutesAlreadyExists
		}

		buf, err := json.Marshal(route)
		if err != nil {
			return err
		}

		err = b.Put(routePath, buf)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return route, nil
}

func (ds *BoltDatastore) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	if newroute == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	if newroute.AppName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if newroute.Path == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	routePath := []byte(newroute.Path)

	var route *models.Route

	err := ds.db.Update(func(tx *bolt.Tx) error {
		b, err := ds.getRouteBucketForApp(tx, newroute.AppName)
		if err != nil {
			return err
		}

		v := b.Get(routePath)
		if v == nil {
			return models.ErrRoutesNotFound
		}

		err = json.Unmarshal(v, &route)
		if err != nil {
			return err
		}
		// Update route fields
		if newroute.Image != "" {
			route.Image = newroute.Image
		}
		if route.Memory != 0 {
			route.Memory = newroute.Memory
		}
		if route.Type != "" {
			route.Type = newroute.Type
		}
		if newroute.Headers != nil {
			if route.Config == nil {
				route.Config = map[string]string{}
			}
			for k, v := range newroute.Headers {
				route.Headers[k] = v
			}
		}
		if newroute.Config != nil {
			if route.Config == nil {
				route.Config = map[string]string{}
			}
			for k, v := range newroute.Config {
				route.Config[k] = v
			}
		}

		buf, err := json.Marshal(route)
		if err != nil {
			return err
		}

		err = b.Put(routePath, buf)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return route, nil
}

func (ds *BoltDatastore) RemoveRoute(ctx context.Context, appName, routePath string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}

	if routePath == "" {
		return models.ErrDatastoreEmptyRoutePath
	}

	err := ds.db.Update(func(tx *bolt.Tx) error {
		b, err := ds.getRouteBucketForApp(tx, appName)
		if err != nil {
			return err
		}

		err = b.Delete([]byte(routePath))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (ds *BoltDatastore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if routePath == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	var route *models.Route
	err := ds.db.View(func(tx *bolt.Tx) error {
		b, err := ds.getRouteBucketForApp(tx, appName)
		if err != nil {
			return err
		}

		v := b.Get([]byte(routePath))
		if v != nil {
			err = json.Unmarshal(v, &route)
		}
		return err
	})
	return route, err
}

func (ds *BoltDatastore) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	err := ds.db.View(func(tx *bolt.Tx) error {
		b, err := ds.getRouteBucketForApp(tx, appName)
		if err != nil {
			return err
		}

		i := 0
		c := b.Cursor()

		var k, v []byte
		k, v = c.Last()

		// Iterate backwards, newest first
		for ; k != nil; k, v = c.Prev() {
			var route models.Route
			err := json.Unmarshal(v, &route)
			if err != nil {
				return err
			}
			if applyRouteFilter(&route, filter) {
				i++
				res = append(res, &route)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *BoltDatastore) GetRoutes(ctx context.Context, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}
	err := ds.db.View(func(tx *bolt.Tx) error {
		i := 0
		rbucket := tx.Bucket(ds.routesBucket)

		b := rbucket.Cursor()
		var k, v []byte
		k, v = b.First()

		// Iterates all buckets
		for ; k != nil && v == nil; k, v = b.Next() {
			bucket := rbucket.Bucket(k)
			r := bucket.Cursor()
			var k2, v2 []byte
			k2, v2 = r.Last()
			// Iterate all routes
			for ; k2 != nil; k2, v2 = r.Prev() {
				var route models.Route
				err := json.Unmarshal(v2, &route)
				if err != nil {
					return err
				}
				if applyRouteFilter(&route, filter) {
					i++
					res = append(res, &route)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *BoltDatastore) Put(ctx context.Context, key, value []byte) error {
	ds.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(ds.extrasBucket) // todo: maybe namespace by app?
		err := b.Put(key, value)
		return err
	})
	return nil
}

func (ds *BoltDatastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	var ret []byte
	ds.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(ds.extrasBucket)
		ret = b.Get(key)
		return nil
	})
	return ret, nil
}

func applyAppFilter(app *models.App, filter *models.AppFilter) bool {
	if filter.Name != "" {
		nameLike, err := regexp.MatchString(strings.Replace(filter.Name, "%", ".*", -1), app.Name)
		return err == nil && nameLike
	}

	return true
}

func applyRouteFilter(route *models.Route, filter *models.RouteFilter) bool {
	return (filter.Path == "" || route.Path == filter.Path) &&
		(filter.AppName == "" || route.AppName == filter.AppName) &&
		(filter.Image == "" || route.Image == filter.Image)
}
