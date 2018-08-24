// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestListVolumes(t *testing.T) {
	t.Parallel()
	volumesData := `[
	{
		"Name": "tardis",
		"Driver": "local",
		"Mountpoint": "/var/lib/docker/volumes/tardis",
		"CreatedAt": "2017-07-19T12:00:26Z"
	},
	{
		"Name": "foo",
		"Driver": "bar",
		"Mountpoint": "/var/lib/docker/volumes/bar",
		"CreatedAt": "2017-07-19T12:01:26Z"
	}
]`
	body := `{ "Volumes": ` + volumesData + ` }`
	var expected []Volume
	if err := json.Unmarshal([]byte(volumesData), &expected); err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: body, status: http.StatusOK})
	volumes, err := client.ListVolumes(ListVolumesOptions{})
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(volumes, expected) {
		t.Errorf("ListVolumes: Wrong return value. Want %#v. Got %#v.", expected, volumes)
	}
}

func TestCreateVolume(t *testing.T) {
	t.Parallel()
	body := `{
		"Name": "tardis",
		"Driver": "local",
		"Mountpoint": "/var/lib/docker/volumes/tardis"
	}`
	var expected Volume
	if err := json.Unmarshal([]byte(body), &expected); err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: body, status: http.StatusOK}
	client := newTestClient(fakeRT)
	volume, err := client.CreateVolume(
		CreateVolumeOptions{
			Name:   "tardis",
			Driver: "local",
			DriverOpts: map[string]string{
				"foo": "bar",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(volume, &expected) {
		t.Errorf("CreateVolume: Wrong return value. Want %#v. Got %#v.", expected, volume)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("CreateVolume(): Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/volumes/create"))
	if req.URL.Path != u.Path {
		t.Errorf("CreateVolume(): Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestInspectVolume(t *testing.T) {
	t.Parallel()
	body := `{
		"Name": "tardis",
		"Driver": "local",
		"Mountpoint": "/var/lib/docker/volumes/tardis",
		"Options": {
			"foo": "bar"
		}
	}`
	var expected Volume
	if err := json.Unmarshal([]byte(body), &expected); err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: body, status: http.StatusOK}
	client := newTestClient(fakeRT)
	name := "tardis"
	volume, err := client.InspectVolume(name)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(volume, &expected) {
		t.Errorf("InspectVolume: Wrong return value. Want %#v. Got %#v.", expected, volume)
	}
	req := fakeRT.requests[0]
	expectedMethod := "GET"
	if req.Method != expectedMethod {
		t.Errorf("InspectVolume(%q): Wrong HTTP method. Want %s. Got %s.", name, expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/volumes/" + name))
	if req.URL.Path != u.Path {
		t.Errorf("CreateVolume(%q): Wrong request path. Want %q. Got %q.", name, u.Path, req.URL.Path)
	}
}

func TestRemoveVolume(t *testing.T) {
	t.Parallel()
	name := "test"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	if err := client.RemoveVolume(name); err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "DELETE"
	if req.Method != expectedMethod {
		t.Errorf("RemoveVolume(%q): Wrong HTTP method. Want %s. Got %s.", name, expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/volumes/" + name))
	if req.URL.Path != u.Path {
		t.Errorf("RemoveVolume(%q): Wrong request path. Want %q. Got %q.", name, u.Path, req.URL.Path)
	}
}

func TestRemoveVolumeWithOptions(t *testing.T) {
	t.Parallel()
	name := "test"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	if err := client.RemoveVolumeWithOptions(RemoveVolumeOptions{
		Name:  name,
		Force: true,
	}); err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "DELETE"
	if req.Method != expectedMethod {
		t.Errorf("RemoveVolume(%q): Wrong HTTP method. Want %s. Got %s.", name, expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/volumes/" + name + "?force=1"))
	if req.URL.RequestURI() != u.RequestURI() {
		t.Errorf("RemoveVolume(%q): Wrong request path. Want %q. Got %q.", name, u.RequestURI(), req.URL.RequestURI())
	}
}

func TestRemoveVolumeNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such volume", status: http.StatusNotFound})
	if err := client.RemoveVolume("test:"); err != ErrNoSuchVolume {
		t.Errorf("RemoveVolume: wrong error. Want %#v. Got %#v.", ErrNoSuchVolume, err)
	}
}

func TestRemoveVolumeInternalError(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "something went wrong", status: http.StatusInternalServerError})
	if err := client.RemoveVolume("test:test"); err == nil {
		t.Error("RemoveVolume: unexpected <nil> error")
	}
}

func TestRemoveVolumeInUse(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "volume in use and cannot be removed", status: http.StatusConflict})
	if err := client.RemoveVolume("test:"); err != ErrVolumeInUse {
		t.Errorf("RemoveVolume: wrong error. Want %#v. Got %#v.", ErrVolumeInUse, err)
	}
}

func TestPruneVolumes(t *testing.T) {
	t.Parallel()
	results := `{
		"VolumesDeleted": [
			"a", "b", "c"
		],
		"SpaceReclaimed": 123
	}`

	expected := &PruneVolumesResults{}
	err := json.Unmarshal([]byte(results), expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: results, status: http.StatusOK})
	got, err := client.PruneVolumes(PruneVolumesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("PruneContainers: Expected %#v. Got %#v.", expected, got)
	}
}
