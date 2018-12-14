package grpcutil

import (
	"context"
	"net"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/sirupsen/logrus"
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
		log := common.Logger(ctx).WithField("grpc_addr", address)

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		conn, err := (&net.Dialer{Cancel: ctx.Done(), Timeout: timeoutDialer}).Dial("tcp", address)
		if err != nil {
			log.WithError(err).Debug("Failed to dial grpc connection")
			return nil, err
		}
		if creds == nil {
			log.Warn("Created insecure grpc connection")
			return conn, nil
		}

		conn, _, err = creds.ClientHandshake(ctx, address, conn)
		if err != nil {
			log.Debug("Failed grpc TLS handshake")
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

// RIDStreamServerInterceptor is a gRPC stream interceptor which gets the request ID out of the context and put a logger with request ID logged into the common logger in the context
func RIDStreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	newStream := grpc_middleware.WrapServerStream(stream)
	rid := ridFromMetadata(stream.Context())
	if rid != "" {
		newStream.WrappedContext, _ = common.LoggerWithFields(newStream.WrappedContext, logrus.Fields{common.RequestIDContextKey: rid})
	}
	return handler(srv, newStream)
}

// RIDUnaryServerInterceptor is an unary gRPC interceptor which gets the request ID out of the context and put a logger with request ID logged into the common logger in the context
func RIDUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	rid := ridFromMetadata(ctx)
	if rid != "" {
		ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{common.RequestIDContextKey: rid})
	}
	return handler(ctx, req)
}

func ridFromMetadata(ctx context.Context) string {
	rid := ""
	md, ok := metadata.FromIncomingContext(ctx)
	if ok && len(md[common.RequestIDContextKey]) > 0 {
		rid = md[common.RequestIDContextKey][0]
	}
	return rid
}
