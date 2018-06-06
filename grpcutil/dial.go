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

	"github.com/fnproject/fn/api/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DialWithBackoff creates a grpc connection using backoff strategy for reconnections
func DialWithBackoff(ctx context.Context, address string, creds credentials.TransportCredentials, timeout time.Duration, backoffCfg grpc.BackoffConfig) (*grpc.ClientConn, error) {
	return dial(ctx, address, creds, timeout, grpc.WithBackoffConfig(backoffCfg))
}

// uses grpc connection backoff protocol https://github.com/grpc/grpc/blob/master/doc/connection-backoff.md
func dial(ctx context.Context, address string, creds credentials.TransportCredentials, timeoutDialer time.Duration, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialer := func(address string, timeout time.Duration) (net.Conn, error) {
		log := common.Logger(ctx).WithField("grpc_addr", address)

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		conn, err := (&net.Dialer{Cancel: ctx.Done(), Timeout: timeoutDialer}).Dial("tcp", address)
		if err != nil {
			log.WithError(err).Warn("Failed to dial grpc connection")
			return nil, err
		}
		if creds == nil {
			log.Warn("Created insecure grpc connection")
			return conn, nil
		}

		conn, _, err = creds.ClientHandshake(ctx, address, conn)
		if err != nil {
			log.Warn("Failed grpc TLS handshake")
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
