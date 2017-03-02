package redis

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"context"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/iron-io/functions/api/models"
)

type RedisDataStore struct {
	conn redis.Conn
}

func New(url *url.URL) (models.Datastore, error) {
	pool := &redis.Pool{
		MaxIdle: 4,
		// I'm not sure if allowing the pool to block if more than 16 connections are required is a good idea.
		MaxActive:   16,
		Wait:        true,
		IdleTimeout: 300 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.DialURL(url.String())
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	// Force a connection so we can fail in case of error.
	conn := pool.Get()

	if err := conn.Err(); err != nil {
		logrus.WithError(err).Fatal("Error connecting to redis")
	}
	ds := &RedisDataStore{
		conn: conn,
	}
	return ds, nil
}

func (ds *RedisDataStore) setApp(app *models.App) (*models.App, error) {
	appBytes, err := json.Marshal(app)
	if err != nil {
		return nil, err
	}

	if _, err := ds.conn.Do("HSET", "apps", app.Name, appBytes); err != nil {
		return nil, err
	}
	return app, nil
}

func (ds *RedisDataStore) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	reply, err := ds.conn.Do("HEXISTS", "apps", app.Name)
	if err != nil {
		return nil, err
	}
	if exists, err := redis.Bool(reply, err); err != nil {
		return nil, err
	} else if exists {
		return nil, models.ErrAppsAlreadyExists
	}

	return ds.setApp(app)
}

func (ds *RedisDataStore) UpdateApp(ctx context.Context, newapp *models.App) (*models.App, error) {
	if newapp == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if newapp.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	app, err := ds.GetApp(ctx, newapp.Name)
	if err != nil {
		return nil, err
	}

	app.UpdateConfig(newapp.Config)

	return ds.setApp(app)
}

func (ds *RedisDataStore) RemoveApp(ctx context.Context, appName string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}

	if _, err := ds.conn.Do("HDEL", "apps", appName); err != nil {
		return err
	}

	return nil
}

func (ds *RedisDataStore) GetApp(ctx context.Context, name string) (*models.App, error) {
	if name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	reply, err := ds.conn.Do("HGET", "apps", name)
	if err != nil {
		return nil, err
	} else if reply == nil {
		return nil, models.ErrAppsNotFound
	}

	res := &models.App{}
	if err := json.Unmarshal(reply.([]byte), res); err != nil {
		return nil, err
	}

	return res, nil
}

func (ds *RedisDataStore) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	res := []*models.App{}

	reply, err := ds.conn.Do("HGETALL", "apps")
	if err != nil {
		return nil, err
	}

	apps, err := redis.StringMap(reply, err)
	if err != nil {
		return nil, err
	}

	for _, v := range apps {
		var app models.App
		if err := json.Unmarshal([]byte(v), &app); err != nil {
			return nil, err
		}
		if applyAppFilter(&app, filter) {
			res = append(res, &app)
		}
	}
	return res, nil
}

func (ds *RedisDataStore) setRoute(set string, route *models.Route) (*models.Route, error) {
	buf, err := json.Marshal(route)
	if err != nil {
		return nil, err
	}

	if _, err := ds.conn.Do("HSET", set, route.Path, buf); err != nil {
		return nil, err
	}

	return route, nil
}

func (ds *RedisDataStore) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	if route.AppName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if route.Path == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	reply, err := ds.conn.Do("HEXISTS", "apps", route.AppName)
	if err != nil {
		return nil, err
	}
	if exists, err := redis.Bool(reply, err); err != nil {
		return nil, err
	} else if !exists {
		return nil, models.ErrAppsNotFound
	}

	hset := fmt.Sprintf("routes:%s", route.AppName)

	reply, err = ds.conn.Do("HEXISTS", hset, route.Path)
	if err != nil {
		return nil, err
	}

	if exists, err := redis.Bool(reply, err); err != nil {
		return nil, err
	} else if exists {
		return nil, models.ErrRoutesAlreadyExists
	}

	return ds.setRoute(hset, route)
}

func (ds *RedisDataStore) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	if newroute == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	if newroute.AppName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if newroute.Path == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	route, err := ds.GetRoute(ctx, newroute.AppName, newroute.Path)
	if err != nil {
		return nil, err
	}

	route.Update(newroute)


	hset := fmt.Sprintf("routes:%s", route.AppName)

	return ds.setRoute(hset, route)
}

func (ds *RedisDataStore) RemoveRoute(ctx context.Context, appName, routePath string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}

	if routePath == "" {
		return models.ErrDatastoreEmptyRoutePath
	}

	hset := fmt.Sprintf("routes:%s", appName)
	if n, err := ds.conn.Do("HDEL", hset, routePath); err != nil {
		return err
	} else if n == 0 {
		return models.ErrRoutesRemoving
	}

	return nil
}

func (ds *RedisDataStore) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if routePath == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	hset := fmt.Sprintf("routes:%s", appName)
	reply, err := ds.conn.Do("HGET", hset, routePath)
	if err != nil {
		return nil, err
	} else if reply == nil {
		return nil, models.ErrRoutesNotFound
	}

	var route models.Route
	if err := json.Unmarshal(reply.([]byte), &route); err != nil {
		return nil, err
	}

	return &route, nil
}

func (ds *RedisDataStore) GetRoutes(ctx context.Context, filter *models.RouteFilter) ([]*models.Route, error) {
	res := []*models.Route{}

	reply, err := ds.conn.Do("HKEYS", "apps")
	if err != nil {
		return nil, err
	} else if reply == nil {
		return nil, models.ErrRoutesNotFound
	}
	paths, err := redis.Strings(reply, err)

	for _, path := range paths {
		hset := fmt.Sprintf("routes:%s", path)
		reply, err := ds.conn.Do("HGETALL", hset)
		if err != nil {
			return nil, err
		} else if reply == nil {
			return nil, models.ErrRoutesNotFound
		}
		routes, err := redis.StringMap(reply, err)

		for _, v := range routes {
			var route models.Route
			if err := json.Unmarshal([]byte(v), &route); err != nil {
				return nil, err
			}
			if applyRouteFilter(&route, filter) {
				res = append(res, &route)
			}
		}
	}

	return res, nil
}

func (ds *RedisDataStore) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) ([]*models.Route, error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	if filter == nil {
		filter = new(models.RouteFilter)
	}
	filter.AppName = appName
	res := []*models.Route{}

	hset := fmt.Sprintf("routes:%s", appName)
	reply, err := ds.conn.Do("HGETALL", hset)
	if err != nil {
		return nil, err
	} else if reply == nil {
		return nil, models.ErrRoutesNotFound
	}
	routes, err := redis.StringMap(reply, err)

	for _, v := range routes {
		var route models.Route
		if err := json.Unmarshal([]byte(v), &route); err != nil {
			return nil, err
		}
		if applyRouteFilter(&route, filter) {
			res = append(res, &route)
		}
	}

	return res, nil
}

func (ds *RedisDataStore) Put(ctx context.Context, key, value []byte) error {
	if key == nil || len(key) == 0 {
		return models.ErrDatastoreEmptyKey
	}

	if _, err := ds.conn.Do("HSET", "extras", key, value); err != nil {
		return err
	}

	return nil
}

func (ds *RedisDataStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	if key == nil || len(key) == 0 {
		return nil, models.ErrDatastoreEmptyKey
	}

	value, err := ds.conn.Do("HGET", "extras", key)
	if err != nil {
		return nil, err
	}

	return value.([]byte), nil
}

func applyAppFilter(app *models.App, filter *models.AppFilter) bool {
	if filter != nil && filter.Name != "" {
		nameLike, err := regexp.MatchString(strings.Replace(filter.Name, "%", ".*", -1), app.Name)
		return err == nil && nameLike
	}

	return true
}

func applyRouteFilter(route *models.Route, filter *models.RouteFilter) bool {
	return filter == nil || (filter.Path == "" || route.Path == filter.Path) &&
		(filter.AppName == "" || route.AppName == filter.AppName) &&
		(filter.Image == "" || route.Image == filter.Image)
}
