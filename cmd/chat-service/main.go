// Command chat-service запускает gRPC-сервер мессенджера.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcserver "github.com/Ippolid/messenger/internal/transport/grpc"

	"github.com/Ippolid/messenger/internal/auth"
	"github.com/Ippolid/messenger/internal/storage"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := envOr("GRPC_ADDR", ":50051")
	dsn := envOr("DB_DSN", "postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable")
	jwtSecret := envOr("JWT_SECRET", "dev-secret-change-me")

	// для graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer store.Close()

	tokens := auth.NewTokenManager(jwtSecret, auth.AccessTTL)
	authSvc := auth.NewService(store, tokens)
	srv := grpcserver.New(authSvc, tokens)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// Останавливаем сервер по сигналу.
	go func() {
		<-ctx.Done()
		log.Println("shutting down gRPC server...")
		srv.GracefulStop()
	}()

	log.Printf("chat-service listening on %s", addr)
	if err := srv.Serve(lis); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

// envOr возвращает значение переменной окружения или дефолт
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
