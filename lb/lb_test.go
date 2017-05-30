package main

import "testing"

func TestCHAdd(t *testing.T) {
	var ch consistentHash
	nodes := []string{"1", "2", "3"}
	for _, n := range nodes {
		ch.add(n)
	}

	if len(ch.nodes) != 3 {
		t.Fatal("nodes list should be len of 3, got:", len(ch.nodes))
	}

	// test dupes don't get added
	for _, n := range nodes {
		ch.add(n)
	}

	if len(ch.nodes) != 3 {
		t.Fatal("nodes list should be len of 3, got:", len(ch.nodes))
	}
}

func TestCHRemove(t *testing.T) {
	var ch consistentHash
	nodes := []string{"1", "2", "3"}
	for _, n := range nodes {
		ch.add(n)
	}

	if len(ch.nodes) != 3 {
		t.Fatal("nodes list should be len of 3, got:", len(ch.nodes))
	}

	ch.remove("4")

	if len(ch.nodes) != 3 {
		t.Fatal("nodes list should be len of 3, got:", len(ch.nodes))
	}

	ch.remove("3")

	if len(ch.nodes) != 2 {
		t.Fatal("nodes list should be len of 2, got:", len(ch.nodes))
	}

	ch.remove("3")

	if len(ch.nodes) != 2 {
		t.Fatal("nodes list should be len of 2, got:", len(ch.nodes))
	}
}

func TestCHGet(t *testing.T) {
	var ch consistentHash
	nodes := []string{"1", "2", "3"}
	for _, n := range nodes {
		ch.add(n)
	}

	if len(ch.nodes) != 3 {
		t.Fatal("nodes list should be len of 3, got:", len(ch.nodes))
	}

	keys := []string{"a", "b", "c"}
	for _, k := range keys {
		_, err := ch.get(k)
		if err != nil {
			t.Fatal("CHGet returned an error: ", err)
		}
		// testing this doesn't panic basically? could test distro but meh
	}
}
