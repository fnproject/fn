package grpcutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/fnproject/fn/api/common"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// DialWithBackoff creates a grpc connection using backoff strategy for reconnections
func DialWithBackoff(ctx context.Context, address string, creds credentials.TransportCredentials, timeout time.Duration, backoffCfg grpc.BackoffConfig, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts = append(opts, grpc.WithBackoffConfig(backoffCfg))
	return dial(ctx, address, creds, timeout, opts...)
}

// uses grpc connection backoff protocol https://github.com/grpc/grpc/blob/master/doc/connection-backoff.md
func dial(ctx context.Context, address string, creds credentials.TransportCredentials, timeoutDialer time.Duration, opts ...grpc.DialOption) (*grpc.ClientConn, error) {

	dialer := func(address string, timeout time.Duration) (net.Conn, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		conn, err := (&net.Dialer{Cancel: ctx.Done(), Timeout: timeoutDialer}).Dial("tcp", address)
		if err != nil {
			logrus.WithError(err).WithField("grpc_addr", address).Warn("Failed to dial grpc connection")
			return nil, err
		}
		if creds == nil {
			logrus.WithField("grpc_addr", address).Warn("Created insecure grpc connection")
			return conn, nil
		}

		conn, _, err = creds.ClientHandshake(ctx, address, conn)
		if err != nil {
			logrus.WithError(err).WithField("grpc_addr", address).Warn("Failed grpc TLS handshake")
			return nil, err
		}
		return conn, nil
	}

	opts = append(opts,
		grpc.WithDialer(dialer),
		grpc.WithInsecure(), // we are handling TLS, so tell grpc not to
	)
	return grpc.DialContext(ctx, address, opts...)

}

// CreateCredentials creates a new set of TLS credentials
// certificateCommonName must match the CN of the signed certificate
// for the TLS handshake to work
func CreateCredentials(certPath, keyPath, caCertPath, certCommonName string) (credentials.TransportCredentials, error) {
	// Load the client certificates from disk
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}

	// Append the certificates from the CA
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.New("failed to append ca certs")
	}

	return credentials.NewTLS(&tls.Config{
		ServerName:   certCommonName,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	}), nil
}

// RIDStreamServerInterceptor is a gRPC stream interceptor which gets the request ID out of the context and put a logger with request ID logged into the common logger in the context
func RIDStreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	newStream := grpc_middleware.WrapServerStream(stream)
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ridKeyToString := string(common.RIDContextKey())
		rid := md[ridKeyToString][0]
		newStream.WrappedContext, _ = common.LoggerWithFields(newStream.WrappedContext, logrus.Fields{ridKeyToString: rid})
	}
	return handler(srv, newStream)
}

// RIDUnaryServerInterceptor is an unary gRPC interceptor which gets the request ID out of the context and put a logger with request ID logged into the common logger in the context
func RIDUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ridKeyToString := string(common.RIDContextKey())
		rid := md[ridKeyToString][0]
		ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{ridKeyToString: rid})
	}
	return handler(ctx, req)
}
