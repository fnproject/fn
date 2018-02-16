package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	runner "github.com/fnproject/fn/api/agent/grpc"
	//"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	//"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
)

type pureRunner struct {
	gRPCServer *grpc.Server
	listen     string
	a          Agent
}

type runnerClient struct {
	client    runner.RunnerProtocol_EngageServer
	callState sync.Map
	id        string
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
	log.Println("committing call with headers  %v : %d", w.outHeaders, w.outStatus)

	var outHeaders []*runner.HttpHeader

	for h, vals := range w.outHeaders {
		for _, v := range vals {
			outHeaders = append(outHeaders, &runner.HttpHeader{
				Key:   h,
				Value: v,
			})
		}
	}

	log.Println("sending response value")

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
		log.Println("error sending call", err)
		panic(err)
	}
	log.Println("Sent call response ")
}

func (w *writerFacade) Write(data []byte) (int, error) {
	log.Printf("Got response data %s", string(data))
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
	c       *call // the agent's version of call
	w       *writerFacade
	input   io.WriteCloser
	started bool
	errch   chan error
}

func (pr *pureRunner) handleData(ctx context.Context, data *runner.DataFrame, state *callState) error {
	if !state.started {
		state.started = true
		go func() {
			err := pr.a.Submit(state.c)
			if err != nil {
				state.w.engagement.Send(&runner.RunnerMsg{
					Body: &runner.RunnerMsg_Finished{&runner.CallFinished{
						Success: false,
						Details: fmt.Sprintf("%v", err),
					}}})
			} else {
				state.w.engagement.Send(&runner.RunnerMsg{
					Body: &runner.RunnerMsg_Finished{&runner.CallFinished{
						Success: true,
						Details: state.c.Model().ID,
					}}})
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

	var w http.ResponseWriter
	w = state.w
	inR, inW := io.Pipe()
	agent_call, err := pr.a.GetCall(FromModelAndInput(&c, inR), WithWriter(w), WithReservedSlot(ctx))
	if err != nil {
		return err
	}
	state.c = agent_call.(*call)
	state.input = inW

	return nil
}

// Handles a client engagement
func (pr *pureRunner) Engage(engagement runner.RunnerProtocol_EngageServer) error {
	pv, ok := peer.FromContext(engagement.Context())
	authInfo := pv.AuthInfo.(credentials.TLSInfo)
	clientCn := authInfo.State.PeerCertificates[0].Subject.CommonName
	log.Println("Got connection from", clientCn)
	if ok {
		log.Println("got peer ", pv)
	}
	md, ok := metadata.FromIncomingContext(engagement.Context())
	if ok {
		log.Println("got md ", md)
	}

	var state = callState{
		c: nil,
		w: &writerFacade{
			engagement:    engagement,
			outStatus:     200,
			headerWritten: false,
		},
		started: false,
		errch:   make(chan error),
	}

	grpc.EnableTracing = false
	log.Printf("entering runner loop")
	for {
		msg, err := engagement.Recv()
		if err != nil {
			// Caller may have died, release any slot we've reserved and stop
			// the call.
			if state.c != nil && state.c.reservedSlot != nil {
				state.c.reservedSlot.Close(engagement.Context())
			}
			log.Fatal("io error from server", err)
		}

		switch body := msg.Body.(type) {

		case *runner.ClientMsg_Try:
			err := pr.handleTryCall(engagement.Context(), body.Try, &state)
			if err != nil {
				engagement.Send(&runner.RunnerMsg{
					Body: &runner.RunnerMsg_Acknowledged{&runner.CallAcknowledged{
						Committed: false,
						Details:   fmt.Sprintf("%v", err),
					}}})
			} else {
				engagement.Send(&runner.RunnerMsg{
					Body: &runner.RunnerMsg_Acknowledged{&runner.CallAcknowledged{
						Committed: true,
						Details:   state.c.Model().ID,
					}}})
			}

		case *runner.ClientMsg_Data:
			// TODO If it's the first one, actually start the call. Then stream into current call.
			err := pr.handleData(engagement.Context(), body.Data, &state)
			if err != nil {
				log.Fatal("What do we do here?!?")
			}
		default:
			log.Fatal("Unrecognized or unhandled message")
		}
	}
	return nil
}

func (pr *pureRunner) Start() error {
	log.Println("Listening on ", pr.listen)
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
