package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	runner "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type pureRunner struct {
	gRPCServer  *grpc.Server
	listen      string
	a           Agent
	streamError error // Last communication error on the stream
}

type writerFacade struct {
	engagement    runner.RunnerProtocol_EngageServer
	outHeaders    http.Header
	outStatus     int
	headerWritten bool
}

func (w *writerFacade) Header() http.Header {
	return w.outHeaders
}

func (w *writerFacade) WriteHeader(status int) {
	w.outStatus = status
	w.commitHeaders()
}

func (w *writerFacade) commitHeaders() {
	if w.headerWritten {
		return
	}
	w.headerWritten = true
	logrus.Info("committing call result with headers  %v : %d", w.outHeaders, w.outStatus)

	var outHeaders []*runner.HttpHeader

	for h, vals := range w.outHeaders {
		for _, v := range vals {
			outHeaders = append(outHeaders, &runner.HttpHeader{
				Key:   h,
				Value: v,
			})
		}
	}

	logrus.Info("sending response value")

	err := w.engagement.Send(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_ResultStart{
			ResultStart: &runner.CallResultStart{
				Meta: &runner.CallResultStart_Http{
					Http: &runner.HttpRespMeta{
						Headers:    outHeaders,
						StatusCode: int32(w.outStatus),
					},
				},
			},
		},
	})

	if err != nil {
		logrus.Info("error sending call result", err)
		panic(err)
	}
	logrus.Info("Sent call result response")
}

func (w *writerFacade) Write(data []byte) (int, error) {
	logrus.Debug("Got response data %d bytes long", len(data))
	w.commitHeaders()
	err := w.engagement.Send(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Data: data,
				Eof:  false,
			},
		},
	})

	if err != nil {
		return 0, errors.New("error sending data")
	}
	return len(data), nil
}

func (w *writerFacade) Close() error {
	w.commitHeaders()
	err := w.engagement.Send(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Eof: true,
			},
		},
	})

	if err != nil {
		return errors.New("error sending close frame")
	}
	return nil
}

type callState struct {
	c             *call // the agent's version of call
	w             *writerFacade
	input         io.WriteCloser
	started       bool
	receivedTime  strfmt.DateTime // When was the call received?
	allocatedTime strfmt.DateTime // When did we finish allocating the slot?
	errch         chan error
}

func (pr *pureRunner) handleData(ctx context.Context, data *runner.DataFrame, state *callState) error {
	if !state.started {
		state.started = true
		go func() {
			err := pr.a.Submit(state.c)
			if err != nil {
				if pr.streamError == nil { // If we can still write back...
					err2 := state.w.engagement.Send(&runner.RunnerMsg{
						Body: &runner.RunnerMsg_Finished{&runner.CallFinished{
							Success: false,
							Details: fmt.Sprintf("%v", err),
						}}})
					if err2 != nil {
						pr.streamError = err2
					}
				}
			} else {
				if pr.streamError == nil { // If we can still write back...
					err2 := state.w.engagement.Send(&runner.RunnerMsg{
						Body: &runner.RunnerMsg_Finished{&runner.CallFinished{
							Success: true,
							Details: state.c.Model().ID,
						}}})
					if err2 != nil {
						pr.streamError = err2
					}
				}
			}
		}()
	}

	if len(data.Data) > 0 {
		_, err := state.input.Write(data.Data)
		if err != nil {
			return err
		}
	}
	if data.Eof {
		state.input.Close()
	}
	return nil
}

func (pr *pureRunner) handleTryCall(ctx context.Context, tc *runner.TryCall, state *callState) error {
	var c models.Call
	err := json.Unmarshal([]byte(tc.ModelsCallJson), &c)
	if err != nil {
		return err
	}
	// TODO Validation of the call

	state.receivedTime = strfmt.DateTime(time.Now())
	var w http.ResponseWriter
	w = state.w
	inR, inW := io.Pipe()
	agent_call, err := pr.a.GetCall(FromModelAndInput(&c, inR), WithWriter(w), WithReservedSlot(ctx))
	if err != nil {
		return err
	}
	state.c = agent_call.(*call)
	state.input = inW
	// We spent some time pre-reserving a slot in GetCall so note this down now
	state.allocatedTime = strfmt.DateTime(time.Now())

	return nil
}

// Handles a client engagement
func (pr *pureRunner) Engage(engagement runner.RunnerProtocol_EngageServer) error {
	pv, ok := peer.FromContext(engagement.Context())
	authInfo := pv.AuthInfo.(credentials.TLSInfo)
	clientCn := authInfo.State.PeerCertificates[0].Subject.CommonName
	logrus.Info("Got connection from", clientCn)
	if ok {
		logrus.Info("got peer ", pv)
	}
	md, ok := metadata.FromIncomingContext(engagement.Context())
	if ok {
		logrus.Info("got md ", md)
	}

	var state = callState{
		c: nil,
		w: &writerFacade{
			engagement:    engagement,
			outHeaders:    make(http.Header),
			outStatus:     200,
			headerWritten: false,
		},
		started: false,
		errch:   make(chan error),
	}

	grpc.EnableTracing = false
	logrus.Info("entering runner loop")
	for {
		msg, err := engagement.Recv()
		if err != nil {
			pr.streamError = err
			// Caller may have died. Entirely kill the container by pushing an
			// eof on the input, even for hot. This ensures that the hot
			// container is not stuck in a state where it is still expecting
			// half the input of the previous call. The error this will likely
			// cause will then release the slot.
			if state.c != nil && state.c.reservedSlot != nil {
				state.input.Close()
			}
			return err
		}

		switch body := msg.Body.(type) {

		case *runner.ClientMsg_Try:
			err := pr.handleTryCall(engagement.Context(), body.Try, &state)
			if err != nil {
				if pr.streamError == nil { // If we can still write back...
					err2 := engagement.Send(&runner.RunnerMsg{
						Body: &runner.RunnerMsg_Acknowledged{&runner.CallAcknowledged{
							Committed: false,
							Details:   fmt.Sprintf("%v", err),
						}}})
					if err2 != nil {
						pr.streamError = err2
						return err2
					}
				}
			} else {
				if pr.streamError == nil { // If we can still write back...
					err2 := engagement.Send(&runner.RunnerMsg{
						Body: &runner.RunnerMsg_Acknowledged{&runner.CallAcknowledged{
							Committed:             true,
							Details:               state.c.Model().ID,
							SlotAllocationLatency: time.Time(state.allocatedTime).Sub(time.Time(state.receivedTime)).String(),
						}}})
					if err2 != nil {
						pr.streamError = err2
						return err2
					}
				}
			}

		case *runner.ClientMsg_Data:
			// TODO If it's the first one, actually start the call. Then stream into current call.
			err := pr.handleData(engagement.Context(), body.Data, &state)
			if err != nil {
				// What do we do here?!?
				return err
			}
		default:
			return fmt.Errorf("Unrecognized or unhandled message in receive loop")
		}
	}
	return nil
}

func (pr *pureRunner) Start() error {
	logrus.Info("Listening on ", pr.listen)
	lis, err := net.Listen("tcp", pr.listen)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %s", pr.listen, err)
	}

	if err := pr.gRPCServer.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve error: %s", err)
	}
	return nil
}

func CreatePureRunner(addr string, a Agent, cert string, key string, ca string) (*pureRunner, error) {
	// Load the certificates from disk
	certificate, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("could not load server key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	authority, err := ioutil.ReadFile(ca)
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}

	if ok := certPool.AppendCertsFromPEM(authority); !ok {
		return nil, errors.New("failed to append client certs")
	}

	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	})

	srv := grpc.NewServer(grpc.Creds(creds))

	pr := &pureRunner{
		gRPCServer: srv,
		listen:     addr,
		a:          a,
	}

	runner.RegisterRunnerProtocolServer(srv, pr)

	return pr, nil
}
