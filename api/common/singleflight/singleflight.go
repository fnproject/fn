package singleflight

// TODO figure out how this differs?

// Imported from https://github.com/golang/groupcache/blob/master/singleflight/singleflight.go

import (
	"sync"
)

// call is an in-flight or completed do call
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type SingleFlight struct {
	mu sync.Mutex            // protects m
	m  map[interface{}]*call // lazily initialized
}

// do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
func (g *SingleFlight) Do(key interface{}, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[interface{}]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
