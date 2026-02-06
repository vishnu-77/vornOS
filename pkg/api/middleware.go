package api

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor logs every gRPC request with method, duration, and status.
func LoggingInterceptor(log *zap.SugaredLogger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)

		peerAddr := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			peerAddr = p.Addr.String()
		}

		log.Infow("grpc request",
			"method", info.FullMethod,
			"duration_ms", duration.Milliseconds(),
			"status", code.String(),
			"peer", peerAddr,
		)

		return resp, err
	}
}

// RecoveryInterceptor catches panics in handlers and returns internal errors.
func RecoveryInterceptor(log *zap.SugaredLogger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Errorw("panic in grpc handler",
					"method", info.FullMethod,
					"panic", r,
				)
				err = status.Errorf(13, "internal error") // codes.Internal = 13
			}
		}()
		return handler(ctx, req)
	}
}
