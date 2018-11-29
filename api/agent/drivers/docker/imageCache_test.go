package docker

import (
	"testing"
	docker "github.com/fsouza/go-dockerclient"
	"context"
)

func BasicCache() Cache {
	cacheContext, cacheCancle := context.WithCancel(context.Background())

	imgCache := NewCache(cacheContext, cacheCancle)
	return imgCache
}

func DummyItem(name string) docker.APIImages {
	i:=docker.APIImages{}
	i.ID = name
	return i
}

func TestOneItem(t *testing.T) {
	c := BasicCache()
	if c.Len() != 0 {
		t.Fatal("Incorrect number of items in empty cache")
	}
	c.Add(DummyItem("test"))
	if c.Len() != 1 {
		t.Fatalf("got %v items and expected 1", c.Len())
	}
}

func TestSameItem(t *testing.T) {
	c := BasicCache()
	if c.Len() != 0 {
		t.Fatal("Incorrect number of items in empty cache")
	}
	c.Add(DummyItem("test"))
	if c.Len() != 1 {
		t.Fatalf("got %v items and expected 1", c.Len())
	}

	c.Add(DummyItem("test"))
	if c.Len() != 1 {
		t.Fatalf("got %v items and expected 1", c.Len())
	}


}

func TestTwoItems(t *testing.T) {
	c := BasicCache()
	if c.Len() != 0 {
		t.Fatal("Incorrect number of items in empty cache")
	}
	c.Add(DummyItem("test"))
	if c.Len() != 1 {
		t.Fatalf("got %v items and expected 1", c.Len())
	}

	c.Add(DummyItem("test2"))
	if c.Len() != 2 {
		t.Fatalf("got %v items and expected 2", c.Len())
	}


}

func TestLockAndUnlockOne(t *testing.T) {
	c := BasicCache()
	if c.Len() != 0 {
		t.Fatal("Incorrect number of items in empty cache")
	}
	itm := DummyItem("test")
	c.Add(itm)
	if c.Len() != 1 {
		t.Fatalf("got %v items and expected 1", c.Len())
	}
	c.Lock("test",itm )
	if !c.Locked("test"){
		t.Fatal("Image Should be locked and is not")
	}

	c.Unlock("test",itm )
	if c.Locked("test"){
		t.Fatal("Image Should be unlocked and is not")
	}
}


func TestLockManyOnOne(t *testing.T) {
	c := BasicCache()
	if c.Len() != 0 {
		t.Fatal("Incorrect number of items in empty cache")
	}
	itm := DummyItem("test")
	l1 := 1
	l2 := 2
	l3 := 3
	c.Add(itm)
	if c.Len() != 1 {
		t.Fatalf("got %v items and expected 1", c.Len())
	}
	c.Lock("test",l1)
	if !c.Locked("test"){
		t.Fatal("Image Should be locked and is not")
	}

	c.Lock("test",l2)
	if !c.Locked("test"){
		t.Fatal("Image Should be locked and is not")
	}

	c.Lock("test",l3)
	if !c.Locked("test"){
		t.Fatal("Image Should be locked and is not")
	}

	c.Unlock("test", l1)
	if !c.Locked("test"){
		t.Fatal("Image Should be locked and is not")
	}
	c.Unlock("test", l2)
	if !c.Locked("test"){
		t.Fatal("Image Should be locked and is not")
	}
	c.Unlock("test", l3)
	if !c.Locked("test"){
		t.Fatal("Image Should be locked and is not")
	}
}
