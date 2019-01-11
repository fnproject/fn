package docker

import (
	"context"
	"testing"
	"time"
)

func isNotifySet(ctx context.Context, notify <-chan struct{}) bool {
	select {
	case <-notify:
		return true
	case <-ctx.Done():
	case <-time.After(600 * time.Millisecond):
	}
	return false
}

func TestImageCacherBasic1(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	blTag := "zoo"
	obj := NewImageCache([]string{blTag}, 20)
	rec := obj.GetNotifier()
	inner := obj.(*imageCacher)

	salsa1 := &CachedImage{
		ID:       "salsa1",
		RepoTags: []string{blTag},
		Size:     uint64(25),
	}
	salsa2 := &CachedImage{
		ID:       "salsa2",
		RepoTags: []string{blTag},
		Size:     uint64(25),
	}
	salsa3 := &CachedImage{
		ID:   "salsa3",
		Size: uint64(5),
	}
	salsa4 := &CachedImage{
		ID:   "salsa4",
		Size: uint64(5),
	}

	if obj.IsMaxCapacity() {
		t.Fatalf("empty cache %v over capacity?", inner)
	}

	item := obj.Pop()
	if item != nil {
		t.Fatalf("cache %+v should not Pop(%+v)?", inner, item)
	}

	// Blacklisted images, these all should be no-op
	obj.Update(salsa1)
	obj.MarkBusy(salsa2)
	obj.MarkFree(salsa2)

	if obj.IsMaxCapacity() {
		t.Fatalf("empty cache %v over capacity?", inner)
	}

	item = obj.Pop()
	if item != nil {
		t.Fatalf("cache %+v should not Pop(%+v)?", inner, item)
	}

	if isNotifySet(ctx, rec) {
		t.Fatalf("empty cache %v with notify!", inner)
	}

	// Capacity 0 -> 5
	obj.Update(salsa3)
	// No Capacity change
	obj.MarkBusy(salsa4)

	if obj.IsMaxCapacity() {
		t.Fatalf("cache %v should have 5 < 20 capacity", inner)
	}

	if isNotifySet(ctx, rec) {
		t.Fatalf("cache %v should have 5 < 20 capacity, no ticks", inner)
	}

	// Capacity 5 -> 0
	item = obj.Pop()
	if item == nil || item.ID != "salsa3" {
		t.Fatalf("cache %v should Pop(%+v) salsa3", inner, item)
	}

	// Capacity 0 -> 5
	obj.MarkFree(salsa4)

	item = obj.Pop()
	if item == nil || item.ID != "salsa4" {
		t.Fatalf("cache %v should Pop(%+v) salsa4", inner, item)
	}
}

func TestImageCacherBasic2(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	obj := NewImageCache([]string{}, 20)
	inner := obj.(*imageCacher)
	rec := obj.GetNotifier()

	salsa1 := &CachedImage{
		ID:   "salsa1",
		Size: uint64(25),
	}

	// Capacity 0 -> 25
	obj.Update(salsa1)

	if !obj.IsMaxCapacity() {
		t.Fatalf("cache %v should be over capacity", inner)
	}

	if !isNotifySet(ctx, rec) {
		t.Fatalf("cache %v should have notify!", inner)
	}

	item := obj.Pop()
	if item == nil {
		t.Fatalf("cache %v should Pop", inner)
	}

	item = obj.Pop()
	if item != nil {
		t.Fatalf("cache %+v should not Pop(%+v)?", inner, item)
	}

	if isNotifySet(ctx, rec) {
		t.Fatalf("empty cache %v should have no ticks", inner)
	}
}

func TestImageCacherBasic3(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	obj := NewImageCache([]string{}, 20)
	inner := obj.(*imageCacher)
	rec := obj.GetNotifier()

	img1 := &CachedImage{
		ID:   "salsa1",
		Size: uint64(25),
	}

	obj.Update(img1)

	if !obj.IsMaxCapacity() {
		t.Fatalf("cache %+v should be over capacity", inner)
	}

	if !isNotifySet(ctx, rec) {
		t.Fatalf("cache %+v should have notify!", inner)
	}

	// Busy with ref count 2
	obj.MarkBusy(img1)
	obj.MarkBusy(img1)

	if obj.IsMaxCapacity() {
		t.Fatalf("cache %+v should not be over capacity", inner)
	}

	item := obj.Pop()
	if item != nil {
		t.Fatalf("cache %+v should not Pop(%+v)?", inner, item)
	}

	if isNotifySet(ctx, rec) {
		t.Fatalf("empty cache %+v should have no ticks", inner)
	}

	// ref count 1
	obj.MarkFree(img1)

	if obj.IsMaxCapacity() {
		t.Fatalf("cache %+v should not be over capacity", inner)
	}

	item = obj.Pop()
	if item != nil {
		t.Fatalf("cache %+v should not Pop(%+v)?", inner, item)
	}

	if isNotifySet(ctx, rec) {
		t.Fatalf("empty cache %+v should have no notify", inner)
	}

	// ref count 0 -> adds to LRU
	obj.MarkFree(img1)

	if !obj.IsMaxCapacity() {
		t.Fatalf("cache %+v should be over capacity", inner)
	}

	item = obj.Pop()
	if item == nil {
		t.Fatalf("cache %+v should Pop()", inner)
	}

	if !isNotifySet(ctx, rec) {
		t.Fatalf("cache %+v should have notify!", inner)
	}

	// Should be no-op
	obj.MarkFree(img1)
	obj.MarkFree(img1)

}

func TestImageCacherBasic4(t *testing.T) {
	obj := NewImageCache([]string{}, 20)
	inner := obj.(*imageCacher)

	img1 := &CachedImage{
		ID:   "salsa1",
		Size: uint64(25),
	}

	obj.MarkBusy(img1)

	item := obj.Pop()
	if item != nil {
		t.Fatalf("cache %+v should not Pop(%+v)?", inner, item)
	}

	// This should be a no-op (it is busy)
	obj.Update(img1)

	item = obj.Pop()
	if item != nil {
		t.Fatalf("cache %+v should not Pop(%+v)?", inner, item)
	}

	obj.MarkFree(img1)

	item = obj.Pop()
	if item == nil {
		t.Fatalf("cache %+v should Pop()?", inner)
	}
}
