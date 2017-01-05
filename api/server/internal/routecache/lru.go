// Package routecache is meant to assist in resolving the most used routes at
// an application. Implemented as a LRU, it returns always its full context for
// iteration at the router handler.
package routecache

// based on groupcache's LRU

import (
	"container/list"

	"github.com/iron-io/functions/api/models"
)

// Cache holds an internal linkedlist for hotness management. It is not safe
// for concurrent use, must be guarded externally.
type Cache struct {
	MaxEntries int

	ll    *list.List
	cache map[string]*list.Element
}

// New returns a route cache.
func New(maxentries int) *Cache {
	return &Cache{
		MaxEntries: maxentries,
		ll:         list.New(),
		cache:      make(map[string]*list.Element),
	}
}

// Refresh updates internal linkedlist either adding a new route to the front,
// or moving it to the front when used. It will discard seldom used routes.
func (c *Cache) Refresh(route *models.Route) {
	if c.cache == nil {
		return
	}

	if ee, ok := c.cache[route.AppName+route.Path]; ok {
		c.ll.MoveToFront(ee)
		ee.Value = route
		return
	}

	ele := c.ll.PushFront(route)
	c.cache[route.AppName+route.Path] = ele
	if c.MaxEntries != 0 && c.ll.Len() > c.MaxEntries {
		c.removeOldest()
	}
}

// Get looks up a path's route from the cache.
func (c *Cache) Get(appname, path string) (route *models.Route, ok bool) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[appname+path]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*models.Route), true
	}
	return
}

// Delete removes the element for the given appname and path from the cache.
func (c *Cache) Delete(appname, path string) {
	if ele, hit := c.cache[appname+path]; hit {
		c.removeElement(ele)
	}
}

func (c *Cache) removeOldest() {
	if c.cache == nil {
		return
	}
	if ele := c.ll.Back(); ele != nil {
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*models.Route)
	delete(c.cache, kv.AppName+kv.Path)
}

func (c *Cache) Len() int {
	return len(c.cache)
}
