package models

import (
	"testing"
)

func TestRouteSimple(t *testing.T) {

	route1 := &Route{
		AppName:     "test",
		Path:        "/some",
		Image:       "foo",
		Memory:      128,
		CPUs:        100,
		Type:        "sync",
		Format:      "http",
		Timeout:     10,
		IdleTimeout: 10,
	}

	err := route1.Validate()
	if err != nil {
		t.Fatal("should not have failed, got: ", err)
	}

	route2 := &Route{
		AppName:     "test",
		Path:        "/some",
		Image:       "foo",
		Memory:      128,
		CPUs:        100,
		Type:        "sync",
		Format:      "nonsense",
		Timeout:     10,
		IdleTimeout: 10,
	}

	err = route2.Validate()
	if err == nil {
		t.Fatalf("should have failed route: %#v", route2)
	}
}
