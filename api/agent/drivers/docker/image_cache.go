package docker

import (
	"container/list"
	"sync"
)

// ImageCacher is an image tracker for docker driver. It consists
// of a LRU cache and a reference count map. It keeps of both in-use
// and not in-use images and if the total size exceeds the configured
// limit, then it notifies its consumer. The consumer is expected to
// use IsMaxCapacity()/GetNotifier() functions to keep track of the
// capacity state and is expected to use Pop() function to remove a
// least recently used image from the cache. ImageCacher provides
// Update() to add/update the LRU cache and MarkBusy()/MarkFree()
// function pair to mark/unmark a specific image (reference count)
// in use.

type CachedImage struct {
	ID       string
	ParentID string
	RepoTags []string
	Size     uint64
}

type ImageCacher interface {
	// IsMaxCapacity returns true if total size of all images exceeds the limit
	// and if there's an image in LRU cache that can be removed.
	IsMaxCapacity() bool

	// GetNotifier returns a channel that can be monitored. The channel will return
	// data every time IsMaxCapacity() flips from false to true state.
	GetNotifier() <-chan struct{}

	// Removes an image from the LRU cache if cache is not empty
	Pop() *CachedImage

	// Update adds an image to the LRU cache if the image is not marked in-use
	Update(img *CachedImage)

	// Mark/Unmark an image in-use. If an image is in-use, it will
	// not be a candidate in LRU. When the reference count of the
	// image drops to zero (via MarkFree() calls), the image will
	// be added back to LRU.
	MarkBusy(img *CachedImage)
	MarkFree(img *CachedImage)
}

type imageCacher struct {
	// max total size of images that are in-use or in LRU cache
	maxSize uint64

	// image names/tags that are not eligible for image cacher
	blacklistTags map[string]struct{}

	// notification channel for/when image cacher is over capacity
	// and has an LRU candidate
	notifier chan struct{}

	// big lock for both busy and lru elements below
	lock sync.Mutex

	// LRU for images that are not in-use
	lruSize uint64
	lruList *list.List
	lruMap  map[string]*list.Element

	// reference count of images that are in-use
	busySize uint64
	busyRef  map[string]uint64
}

func NewImageCache(exemptTags []string, maxSize uint64) ImageCacher {
	c := imageCacher{
		maxSize:       maxSize,
		blacklistTags: make(map[string]struct{}),
		notifier:      make(chan struct{}, 1),
		lruList:       list.New(),
		lruMap:        make(map[string]*list.Element),
		busyRef:       make(map[string]uint64),
	}

	for _, tag := range exemptTags {
		c.blacklistTags[tag] = struct{}{}
	}

	return &c
}

// isEligible returns true if the image is eligible to be managed
// by image cacher.
func (c *imageCacher) isEligible(img *CachedImage) bool {
	for _, tag := range img.RepoTags {
		if _, ok := c.blacklistTags[tag]; ok {
			return false
		}
	}
	return true
}

// addLRULocked performs a classic LRU add operation
func (c *imageCacher) addLRULocked(img *CachedImage) {
	ee, ok := c.lruMap[img.ID]
	if ok {
		c.lruList.MoveToFront(ee)
	} else {
		c.lruSize += img.Size
		c.lruMap[img.ID] = c.lruList.PushFront(img)
	}
}

// addLRULocked performs a classic LRU remove operation
func (c *imageCacher) rmLRULocked(img *CachedImage) {
	ee, ok := c.lruMap[img.ID]
	if ok {
		c.lruList.Remove(ee)
		delete(c.lruMap, img.ID)
		c.lruSize -= ee.Value.(*CachedImage).Size
	}
}

// addBusyLocked updates an image in the in-use list and returns true if
// a new insertion to in-use list was performed.
func (c *imageCacher) addBusyLocked(img *CachedImage) bool {
	if ee, ok := c.busyRef[img.ID]; ok {
		c.busyRef[img.ID] = 1 + ee
		return false
	}

	c.busyRef[img.ID] = 1
	c.busySize += img.Size
	return true
}

// rmBusyLocked updates an image in the in-use list and returns true if
// the image is removed from the in-use list.
func (c *imageCacher) rmBusyLocked(img *CachedImage) bool {
	if ee, ok := c.busyRef[img.ID]; ok {
		if ee > 1 {
			c.busyRef[img.ID] = ee - 1
			return false
		}
		delete(c.busyRef, img.ID)
		c.busySize -= img.Size
		return true
	}
	return false
}

// sendNotify tries to wake up any pending listener on notify channel
func (c *imageCacher) sendNotify() {
	select {
	case c.notifier <- struct{}{}:
	default:
	}
}

// We compare both busy + lru size against max. However, we also check if lru is not empty.
// This is because there's no point to show over capacity if Pop() is going to return nil.
func (c *imageCacher) isMaxCapacityLocked() bool {
	return (c.lruSize > 0) && ((c.lruSize + c.busySize) >= c.maxSize)
}

func (c *imageCacher) GetNotifier() <-chan struct{} {
	return c.notifier
}

func (c *imageCacher) IsMaxCapacity() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.isMaxCapacityLocked()
}

// Update adds an image to LRU if the image is not in-use.
func (c *imageCacher) Update(img *CachedImage) {
	if !c.isEligible(img) {
		return
	}

	c.lock.Lock()

	if _, ok := c.busyRef[img.ID]; !ok {
		c.addLRULocked(img)
		if c.isMaxCapacityLocked() {
			defer c.sendNotify()
		}
	}

	c.lock.Unlock()
}

// Pop removes and returns an image from LRU if LRU is not empty
func (c *imageCacher) Pop() *CachedImage {
	c.lock.Lock()
	defer c.lock.Unlock()

	item := c.lruList.Back()
	if item == nil {
		return nil
	}
	img := item.Value.(*CachedImage)
	c.rmLRULocked(img)
	return img
}

// MarkBusy marks an image as in-use and removes it from LRU
func (c *imageCacher) MarkBusy(img *CachedImage) {
	if !c.isEligible(img) {
		return
	}

	c.lock.Lock()

	if c.addBusyLocked(img) {
		c.rmLRULocked(img)
		if c.isMaxCapacityLocked() {
			defer c.sendNotify()
		}
	}

	c.lock.Unlock()
}

// MarkFree marks an image as not in-use and if in-use reference count
// for the image becomes zero, then MarkFree adds the image to the LRU
func (c *imageCacher) MarkFree(img *CachedImage) {
	if !c.isEligible(img) {
		return
	}

	c.lock.Lock()

	if c.rmBusyLocked(img) {
		c.addLRULocked(img)
		if c.isMaxCapacityLocked() {
			defer c.sendNotify()
		}
	}

	c.lock.Unlock()
}
