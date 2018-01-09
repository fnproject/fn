package docker

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	registry "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/fnproject/fn/api/agent/drivers"
	docker "github.com/fsouza/go-dockerclient"
)

var (
	// we need these imported so that they can be unmarshaled properly (yes, docker is mean)
	_ = schema1.SchemaVersion
	_ = schema2.SchemaVersion

	registryTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 2 * time.Minute,
		}).Dial,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(8192),
		},
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConnsPerHost:   32, // TODO tune; we will likely be making lots of requests to same place
		Proxy:                 http.ProxyFromEnvironment,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          512,
		ExpectContinueTimeout: 1 * time.Second,
	}
)

const hubURL = "https://registry.hub.docker.com"

// CheckRegistry will return a sizer, which can be used to check the size of an
// image if the returned error is nil. If the error returned is nil, then
// authentication against the given credentials was successful, if the
// configuration or image do not specify a config.ServerAddress,
// https://hub.docker.com will be tried.  CheckRegistry is a package level
// method since rkt can also use docker images, we may be interested in using
// rkt w/o a docker driver configured; also, we don't have to tote around a
// driver in any tasker that may be interested in registry information (2/2
// cases thus far).
func CheckRegistry(ctx context.Context, image string, config docker.AuthConfiguration) (Sizer, error) {
	regURL, repoName, tag := drivers.ParseImage(image)

	repoNamed, err := reference.WithName(repoName)
	if err != nil {
		return nil, err
	}

	if regURL == "" {
		// image address overrides credential address
		regURL = config.ServerAddress
	}

	regURL, err = registryURL(regURL)
	if err != nil {
		return nil, err
	}

	cm := challenge.NewSimpleManager()

	creds := newCreds(config.Username, config.Password)
	tran := transport.NewTransport(registryTransport,
		auth.NewAuthorizer(cm,
			auth.NewTokenHandler(registryTransport,
				creds,
				repoNamed.Name(),
				"pull",
			),
			auth.NewBasicHandler(creds),
		),
	)

	tran = &retryWrap{cm, tran}

	repo, err := registry.NewRepository(repoNamed, regURL, tran)
	if err != nil {
		return nil, err
	}

	manis, err := repo.Manifests(ctx)
	if err != nil {
		return nil, err
	}

	mani, err := manis.Get(context.TODO(), "", distribution.WithTag(tag))
	if err != nil {
		return nil, err
	}

	blobs := repo.Blobs(ctx)

	// most registries aren't that great, and won't provide a size for the top
	// level digest, so we need to sum up all the layers.  let this be optional
	// with the sizer, since tag is good enough to check existence / auth.

	return &sizer{mani, blobs}, nil
}

type retryWrap struct {
	cm   challenge.Manager
	tran http.RoundTripper
}

func (d *retryWrap) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := d.tran.RoundTrip(req)

	// if it's not authed, we have to add this to the challenge manager,
	// and then retry it (it will get authed and the challenge then accepted).
	// why the docker distribution transport doesn't do this for you is
	// a real testament to what sadists those docker people are.
	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		pingPath := req.URL.Path
		if v2Root := strings.Index(req.URL.Path, "/v2/"); v2Root != -1 {
			pingPath = pingPath[:v2Root+4]
		} else if v1Root := strings.Index(req.URL.Path, "/v1/"); v1Root != -1 {
			pingPath = pingPath[:v1Root] + "/v2/"
		}

		// seriously, we have to rewrite this to the ping path,
		// since looking up challenges strips to this path. YUP. GLHF.
		ogURL := req.URL.Path
		resp.Request.URL.Path = pingPath

		d.cm.AddResponse(resp)

		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()

		// put the original URL path back and try again now...
		req.URL.Path = ogURL
		resp, err = d.tran.RoundTrip(req)
	}
	return resp, err
}

func newCreds(user, pass string) *creds {
	return &creds{m: make(map[string]string), user: user, pass: pass}
}

// implement auth.CredentialStore
type creds struct {
	m          map[string]string
	user, pass string
}

func (c *creds) Basic(u *url.URL) (string, string)                 { return c.user, c.pass }
func (c *creds) RefreshToken(u *url.URL, service string) string    { return c.m[service] }
func (c *creds) SetRefreshToken(u *url.URL, service, token string) { c.m[service] = token }

// Sizer returns size information. This interface is liable to contain more
// than a size at some point, change as needed.
type Sizer interface {
	Size() (int64, error)
}

type sizer struct {
	mani  distribution.Manifest
	blobs distribution.BlobStore
}

func (s *sizer) Size() (int64, error) {
	var sum int64
	for _, r := range s.mani.References() {
		desc, err := s.blobs.Stat(context.TODO(), r.Digest)
		if err != nil {
			return 0, err
		}
		sum += desc.Size
	}
	return sum, nil
}

func registryURL(addr string) (string, error) {
	if addr == "" || strings.Contains(addr, "hub.docker.com") || strings.Contains(addr, "index.docker.io") {
		return hubURL, nil
	}

	uri, err := url.Parse(addr)
	if err != nil {
		return "", err
	}

	if uri.Scheme == "" {
		uri.Scheme = "https"
	}
	uri.Path = strings.TrimSuffix(uri.Path, "/")
	uri.Path = strings.TrimPrefix(uri.Path, "/v2")
	uri.Path = strings.TrimPrefix(uri.Path, "/v1") // just try this, if it fails it fails, not supporting v1
	return uri.String(), nil
}
