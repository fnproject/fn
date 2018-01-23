// THIS SHOULDN'T BE IMPLEMENTED IN A PROTOCOL, BUT IT WAS THE EASIEST PLACE I COULD FIND TO PLUGIN AS THINGS STAND TODAY.

package protocol

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strings"

	"github.com/fnproject/fn/api/agent/drivers"
)

func NewPortHTTP(in io.Writer, out io.Reader, c drivers.ContainerTask) (*PortHTTP, error) {
	fmt.Println("NewPortHTTP, port:", c.Port())
	ph := &PortHTTP{in: in, out: out}
	// setup reverse proxy
	ph.port = c.Port()
	rpURL, err := url.Parse(fmt.Sprintf("http://localhost:%s", c.Port()))
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(rpURL)
	ph.rProxy = rp
	return ph, nil
}

// PortHTTP sends http requests over a port, rather than stdin/out.
type PortHTTP struct {
	// These are the container input streams, not the input from the request or the output for the response
	in     io.Writer
	out    io.Reader
	rProxy *httputil.ReverseProxy
	port   string
}

func (p *PortHTTP) IsStreamable() bool {
	return true
}

func (h *PortHTTP) Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error {
	fmt.Println("about to proxy into container")
	out, err := exec.Command("docker", "ps").CombinedOutput()
	if err != nil {
		return err
	}
	fmt.Println("out:", string(out))

	// write input into container
	rw := w.(http.ResponseWriter) // assuming it's always this for now
	req := ci.Request()
	// strip off /r/myapp
	// todo: should ensure here that this is in local dev mode with /r/myapp, perhaps env var set in gin route matching.
	path := req.URL.Path
	if strings.HasPrefix(path, "/r/") {
		path = path[3:] // remove /r/
		path = path[strings.Index(path, "/")+1:]
		i := strings.Index(path, "/")
		if i >= 0 {
			path = path[i:]
		} else {
			path = ""
		}
		fmt.Println("NEW PATH:", path)
	}
	req.URL.Path = path
	h.rProxy.ServeHTTP(rw, req)
	return nil

	// regular client:
	// resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/hello", h.port))
	// if err != nil {
	// 	// handle error
	// 	fmt.Println("ERROR on GET", err)
	// 	return err
	// }
	// defer resp.Body.Close()
	// body, err := ioutil.ReadAll(resp.Body)
	// fmt.Println("BODY:", string(body))
	// _, err = w.Write(body)
	// return err
}
