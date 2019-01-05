package docker

import (
	"container/list"
	"sync"
)

type CachedImage struct {
	ID       string
	ParentID string
	RepoTags []string
	Size     uint64
}

type ImageCacher interface {
	IsMaxCapacity() bool
	Update(img *CachedImage)
	Pop() *CachedImage
	GetNotifier() chan struct{}

	MarkBusy(img *CachedImage)
	MarkFree(img *CachedImage)
}

type imageCacher struct {
	maxSize     uint64
	blockedTags map[string]struct{}
	notifier    chan struct{}

	// big lock for both busy and lru elements below
	lock sync.Mutex

	lruSize uint64
	lruList *list.List
	lruMap  map[string]*list.Element

	busySize uint64
	busyRef  map[string]uint64
}

func NewImageCache(exemptTags []string, maxSize uint64) ImageCacher {
	c := imageCacher{
		maxSize:     maxSize,
		blockedTags: make(map[string]struct{}),
		notifier:    make(chan struct{}, 1),
		lruList:     list.New(),
		lruMap:      make(map[string]*list.Element),
		busyRef:     make(map[string]uint64),
	}

	for _, tag := range exemptTags {
		c.blockedTags[tag] = struct{}{}
	}

	return &c
}

func (c *imageCacher) isBlocked(img *CachedImage) bool {
	for _, tag := range img.RepoTags {
		if _, ok := c.blockedTags[tag]; ok {
			return true
		}
	}
	return false
}

func (c *imageCacher) addLRULocked(img *CachedImage) {
	ee, ok := c.lruMap[img.ID]
	if ok {
		c.lruList.MoveToFront(ee)
	} else {
		c.lruSize += img.Size
		c.lruMap[img.ID] = c.lruList.PushFront(&img)
	}
}

func (c *imageCacher) rmLRULocked(img *CachedImage) {
	ee, ok := c.lruMap[img.ID]
	if ok {
		c.lruList.Remove(ee)
		delete(c.lruMap, img.ID)
		c.lruSize -= ee.Value.(*CachedImage).Size
	}
}

func (c *imageCacher) sendNotify() {
	select {
	case c.notifier <- struct{}{}:
	default:
	}
}

func (c *imageCacher) GetNotifier() chan struct{} {
	return c.notifier
}

// We compare both busy + lru size against max. However, we also check if lru is not empty.
// This is because there's no point to show over capacity if Pop() is going to return nil.
func (c *imageCacher) isMaxCapacityLocked() bool {
	return (c.lruSize > 0) && ((c.lruSize + c.busySize) >= c.maxSize)
}

func (c *imageCacher) IsMaxCapacity() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.isMaxCapacityLocked()
}

func (c *imageCacher) Update(img *CachedImage) {
	if c.isBlocked(img) {
		return
	}

	c.lock.Lock()
	c.addLRULocked(img)
	doNotify := c.isMaxCapacityLocked()
	c.lock.Unlock()

	if doNotify {
		c.sendNotify()
	}
}

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

func (c *imageCacher) addBusyLocked(img *CachedImage) {
	ee, ok := c.busyRef[img.ID]
	if !ok {
		c.busyRef[img.ID] = 1
		c.busySize += img.Size
	} else {
		c.busyRef[img.ID] = 1 + ee
	}
}

func (c *imageCacher) rmBusyLocked(img *CachedImage) bool {
	ee, ok := c.busyRef[img.ID]
	if !ok {
		return true
	}
	if ee == 1 {
		c.busySize -= img.Size
		delete(c.busyRef, img.ID)
		return true
	}
	c.busyRef[img.ID] = ee - 1
	return false
}

func (c *imageCacher) MarkBusy(img *CachedImage) {
	if c.isBlocked(img) {
		return
	}

	c.lock.Lock()

	c.addBusyLocked(img)
	c.rmLRULocked(img)
	doNotify := c.isMaxCapacityLocked()

	c.lock.Unlock()

	if doNotify {
		c.sendNotify()
	}
}

func (c *imageCacher) MarkFree(img *CachedImage) {
	if c.isBlocked(img) {
		return
	}

	c.lock.Lock()

	if c.rmBusyLocked(img) {
		c.addLRULocked(img)
	}
	doNotify := c.isMaxCapacityLocked()

	c.lock.Unlock()

	if doNotify {
		c.sendNotify()
	}
}
