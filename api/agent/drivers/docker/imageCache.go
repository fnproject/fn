/*
ImageCache holds all the logic for calculating what docker images can be removed from the running agent.
The last used time and the number of uses are both taken into account to calculate a score (timeSinceLastUse/uses)
The higher the score the more evicitable the image is.

ImageCache also provides a method to "lock" an image, insuring it is never deleted. To do so a Lock is called with
the image ID to lock, as well as a token. The token is then added to a set of tokens attached to that entry.
The set is a map of *interface -> *interface where both values are the same.
*/

package docker

import (
	"errors"
	"sort"
	"sync"
	"time"

	d "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
)

// Cache is a list of Entries. Stored as type EntryByAge so that sort works.
// The mutex is present to control access access the list of images and their locks
// The Cache may be used in concurrent contexts
type Cache struct {
	mu      sync.Mutex
	cache   EntryByAge
	maxSize int64
}

// Entry contains when the image was last used, how many times the image has been used,
// The image metadata, and a set of tokens that are locking the image in question.
type Entry struct {
	lastUsed time.Time
	locked   map[*interface{}]*interface{}
	uses     int64
	image    d.APIImages
}

// The score is the time since last use divided by the number of total uses.
func (e Entry) Score() int64 {
	age := time.Now().Sub(e.lastUsed)
	return age.Nanoseconds() / e.uses
}

// Type to wrap the Entry list so that sort works.
type EntryByAge []Entry

func (a EntryByAge) Len() int           { return len(a) }
func (a EntryByAge) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a EntryByAge) Less(i, j int) bool { return a[i].Score() < a[j].Score() }

// NewEntry constructs a an entry from a docker d.APIImages object.
func NewEntry(value d.APIImages) Entry {
	return Entry{
		lastUsed: time.Now(),
		locked:   make(map[*interface{}]*interface{}),
		uses:     0,
		image:    value}
}

// NewCache returns a new cache with the provided maximum items.
func NewCache() *Cache {
	return &Cache{
		cache: make(EntryByAge, 0),
		mu:    sync.Mutex{},
	}
}

// Public method for checking to see if an image in in the list.
// Equlivilance is determined by image.ID. This method locks.
func (c *Cache) Contains(value d.APIImages) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.contains(value)
}

// Helper for contains that does not lock. Used by other imageCache methods internally.
func (c *Cache) contains(value d.APIImages) bool {
	for _, i := range c.cache {
		if i.image.ID == value.ID {
			return true
		}
	}
	return false

}

// Mark marks an image by ID as having been used.
func (c *Cache) Mark(ID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mark(ID)
}

// Internal method for finding the image and adding to it's used count as well as it's lastUsed time.
func (c *Cache) mark(ID string) error {
	for idx, i := range c.cache {
		if i.image.ID == ID {
			c.cache[idx].lastUsed = time.Now()
			c.cache[idx].uses = c.cache[idx].uses + 1
			return nil
		}
	}

	return errors.New("Image not found in cache")
}

// Remove deletes an image from the list. Also grabs a smaller portion of the slice.
func (c *Cache) Remove(value d.APIImages) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for idx, i := range c.cache {
		if i.image.ID == value.ID {
			// Move the last item into the location of the item to be removed
			c.cache[idx] = c.cache[len(c.cache)-1]
			// shorten the list
			c.cache = c.cache[:len(c.cache)-1]
			return nil
		}
	}

	return errors.New("Image not found in cache")
}

// Lock Adds a token to the list of locks on this image by image id.
func (c *Cache) Lock(ID string, key interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lock(ID, key)
}

// Internal method for Lock. Contains no locks.
func (c *Cache) lock(ID string, key interface{}) error {
	for _, i := range c.cache {
		if i.image.ID == ID {
			i.locked[&key] = &key
			return nil
		}
	}
	return errors.New("Image not found in cache")
}

// Check to see if an image by id has any locks on it.
func (c *Cache) Locked(ID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.locked(ID)
}

func (c *Cache) locked(ID string) (bool, error) {
	for _, i := range c.cache {
		if i.image.ID == ID {
			return len(i.locked) > 0, nil
		}
	}
	return false, errors.New("Image not found in cache")
}

// Unlock Removes a token from an image by ID's lock set.
func (c *Cache) Unlock(ID string, key interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unlock(ID, key)
}

func (c *Cache) unlock(ID string, key interface{}) {
	for _, i := range c.cache {
		if i.image.ID == ID {
			delete(i.locked, &key)
		}
	}
}

// Add puts a value into the cache or marks it as used if it is already present. Thread Safe.
func (c *Cache) Add(value d.APIImages) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Debugf("value: %v", value)
	if c.contains(value) {
		c.mark(value.ID)
		return
	}
	c.cache = append(c.cache, NewEntry(value))
}

// Evictable returns all evictable images ordered by score (which is why it's wrapped in EntryByAge)
func (c *Cache) Evictable() EntryByAge {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictable()
}

func (c *Cache) evictable() (ea EntryByAge) {
	for _, i := range c.cache {
		if len(i.locked) == 0 {
			ea = append(ea, i)
		}
	}
	sort.Sort(ea)
	return ea
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.cache)
}
