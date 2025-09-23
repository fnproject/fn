package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/common"
)

func getFakeDocker(t *testing.T) (*httptest.Server, func()) {

	manifestList := `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
   "manifests": [
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "size": 523,
         "digest": "sha256:1c80d00e6877ff57b9b941ab2cbc5bc1058c28294d7068074ccaecb29a1680d3",
         "platform": {
            "architecture": "amd64",
            "os": "linux"
         }
      }
   ]
}`

	manifest := `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/vnd.docker.container.image.v1+json",
      "size": 960,
      "digest": "sha256:16ae518a0268d144203d36a5ba6431e01b9b19eada6e3fe7177af0d4eda025f8"
   },
   "layers": [
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 977,
         "digest": "sha256:1b930d010525941c1d56ec53b97bd057a67ae1865eebf042686d2a2d18271ced"
      }
   ]
}`

	config := `{
	"architecture":"amd64",
	"config":{
		"Cmd":["/hello"],
		"Image":"sha256:a6d1aaad8ca65655449a26146699fe9d61240071f6992975be7e720f1cd42440"
	},
	"container":"8e2caa5a514bb6d8b4f2a2553e9067498d261a0fd83a96aeaaf303943dff6ff9",
	"container_config":{
		"Hostname":"8e2caa5a514b",
		"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"/hello\"]"],
		"Image":"sha256:a6d1aaad8ca65655449a26146699fe9d61240071f6992975be7e720f1cd42440",
		"Labels":{}
	},
	"created":"2019-01-01T01:29:27.650294696Z",
	"docker_version":"18.06.1-ce",
	"history":[
		{"created":"2019-01-01T01:29:27.416803627Z",
		"created_by":"/bin/sh -c #(nop) COPY file:f77490f70ce51da25bd21bfc30cb5e1a24b2b65eb37d4af0c327ddc24f0986a6 in / "
		},
		{"created":"2019-01-01T01:29:27.650294696Z","created_by":"/bin/sh -c #(nop)  CMD [\"/hello\"]"
		,"empty_layer":true
		}
	],
	"os":"linux",
	"rootfs":{
		"type":"layers",
		"diff_ids":["sha256:af0b15c8625bb1938f1d7b17081031f649fd14e6b233688eea3c5483994a66a3"]
	}}`

	// this will fail obviously. Not a valid layer. But this way, we don't need to maintain a valid layer
	// and worry about removing it, always fails scenarios works for us.
	layer := "zoo"

	logStatus := func(r *http.Request, status int) {
		t.Logf("from=%s method=%s proto=%s status=%d host=%s url=%s", r.RemoteAddr, r.Method, r.Proto, status, r.Host, r.URL.String())
	}

	spitError := func(r *http.Request, w http.ResponseWriter, status int) {
		logStatus(r, status)
		w.WriteHeader(status)
		errMsg := `{"errors": [{ "code": "TOOMANYREQUESTS", "message": "garbanzo beans have reached the sky" }]}`
		w.Write([]byte(errMsg))
	}

	var manifestListDone bool
	var manifestDone bool
	var configDone bool
	var layerDone bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.String() == "/v2/" {
			logStatus(r, 200)
			w.WriteHeader(200)
			return
		}

		if r.URL.String() == "/v2/foo/bar/manifests/latest" {
			if !manifestListDone {
				manifestListDone = true
				spitError(r, w, 429)
				return
			}
			w.Header().Set("Content-Type", `application/vnd.docker.distribution.manifest.list.v2+json`)
			w.Header().Set("Docker-Content-Digest",
				`sha256:0553537fa07e2d97debdc40cdf6c6c9d0db7e57591bd86cd7c721f0161542a9e`)
			w.Header().Set("Docker-Distribution-Api-Version", `registry/2.0`)
			logStatus(r, 200)
			w.WriteHeader(200)
			w.Write([]byte(manifestList))
			return
		}

		if r.URL.String() == `/v2/foo/bar/manifests/sha256:1c80d00e6877ff57b9b941ab2cbc5bc1058c28294d7068074ccaecb29a1680d3` {
			if !manifestDone {
				manifestDone = true
				spitError(r, w, 429)
				return
			}
			w.Header().Set("Content-Type", `application/vnd.docker.distribution.manifest.v2+json`)
			w.Header().Set("Docker-Content-Digest",
				`sha256:1c80d00e6877ff57b9b941ab2cbc5bc1058c28294d7068074ccaecb29a1680d3`)
			w.Header().Set("Docker-Distribution-Api-Version", `registry/2.0`)
			logStatus(r, 200)
			w.WriteHeader(200)
			w.Write([]byte(manifest))
			return
		}

		if r.URL.String() == `/v2/foo/bar/blobs/sha256:16ae518a0268d144203d36a5ba6431e01b9b19eada6e3fe7177af0d4eda025f8` {
			if !configDone {
				configDone = true
				spitError(r, w, 429)
				return
			}
			logStatus(r, 200)
			w.WriteHeader(200)
			w.Write([]byte(config))
			return
		}

		if r.URL.String() == `/v2/foo/bar/blobs/sha256:1b930d010525941c1d56ec53b97bd057a67ae1865eebf042686d2a2d18271ced` {
			if !layerDone {
				layerDone = true
				spitError(r, w, 429)
				return
			}
			w.Header().Set("Content-Type", `application/octet-stream`)
			logStatus(r, 200)
			w.WriteHeader(200)
			w.Write([]byte(layer))
			return
		}

		if r.URL.String() == `/v2/foo/bar/manifests/sha256:0553537fa07e2d97debdc40cdf6c6c9d0db7e57591bd86cd7c721f0161542a9e` {
			if !manifestDone {
				manifestDone = true
				spitError(r, w, 429)
				return
			}
			w.Header().Set("Content-Type", `application/vnd.docker.distribution.manifest.v2+json`)
			w.Header().Set("Docker-Content-Digest",
				`sha256:1c80d00e6877ff57b9b941ab2cbc5bc1058c28294d7068074ccaecb29a1680d3`)
			w.Header().Set("Docker-Distribution-Api-Version", `registry/2.0`)
			logStatus(r, 200)
			w.WriteHeader(200)
			w.Write([]byte(manifest))
			return
		}

		logStatus(r, 500)
		w.WriteHeader(500)
		return
	}))

	return srv, func() {
		srv.Close()
	}
}

// Using a fake repo, which returns 429 once for each URI. The pull eventually
// should succeeds if retries work. "Success" here means we get "filesystem verification failed"
// (since we do not actually use a valid layer at the end.)
func TestDockerPullRetries(t *testing.T) {
	dockerSrv, dockerCancel := getFakeDocker(t)
	defer dockerCancel()

	a, d, err := getAgentWithDriver()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	checker := func(err error) (bool, string) {
		if err != nil && strings.Index(err.Error(), "toomanyrequests: ") != -1 {
			return true, "toomanyrequests"
		}
		return false, ""
	}

	policy := common.BackOffConfig{
		MaxRetries: 5,
		Interval:   50,
	}

	err = d.SetPullImageRetryPolicy(policy, checker)
	if err != nil {
		t.Fatal("cannot set retry policy")
	}

	fn := getFn(0)
	fn.Timeout = 10
	fn.Memory = 64
	fn.Image = strings.TrimPrefix(dockerSrv.URL, "https://") + "/foo/bar:latest"

	err = execFn(`{"sleepTime": 0}`, fn, getApp(), a, 400000)
	if err == nil || strings.Index(err.Error(), "filesystem layer verification failed") == -1 {
		t.Fatalf("unexpected error %v", err)
	}
}

// Using a fake repo, which returns 429 once for each URI. The pull should fail since
// retry checker function never returns true, which disables retries.
func TestDockerPullNoRetry(t *testing.T) {

	dockerSrv, dockerCancel := getFakeDocker(t)
	defer dockerCancel()

	a, d, err := getAgentWithDriver()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	checker := func(err error) (bool, string) {
		return false, ""
	}

	policy := common.BackOffConfig{
		MaxRetries: 5,
		Interval:   50,
	}

	err = d.SetPullImageRetryPolicy(policy, checker)
	if err != nil {
		t.Fatal("cannot set retry policy")
	}

	fn := getFn(0)
	fn.Timeout = 10
	fn.Memory = 64
	fn.Image = strings.TrimPrefix(dockerSrv.URL, "https://") + "/foo/bar:latest"

	err = execFn(`{"sleepTime": 0}`, fn, getApp(), a, 400000)
	if err == nil || strings.Index(err.Error(), "garbanzo beans have reached the sky") == -1 {
		t.Fatalf("unexpected error %v", err)
	}
}
