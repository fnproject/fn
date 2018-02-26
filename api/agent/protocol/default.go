package protocol

import (
	"context"
	"io"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct {
	in  io.WriteCloser
	out io.Reader
}

func (p *DefaultProtocol) IsStreamable() bool { return false }

func (d *DefaultProtocol) Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error {

	// Here we perform I/O in their own go routines. We do not assume any ordering such
	// as request/response in 'default' protocol. In other words, output may arrive
	// before we finish providing input.

	errApp := make(chan error, 2)
	go func() {
		_, err := io.Copy(d.in, ci.Input())
		d.in.Close()
		errApp <- err
	}()

	go func() {
		_, err := io.Copy(w, d.out)
		errApp <- err
	}()

	err := <-errApp
	if err == nil {
		err = <-errApp
	}
	return err
}
