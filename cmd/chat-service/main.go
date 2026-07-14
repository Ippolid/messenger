// Command chat-service запускает gRPC-сервер мессенджера.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ippolid/messenger/internal/broker"
	"github.com/Ippolid/messenger/internal/chat"
	"github.com/Ippolid/messenger/internal/ratelimit"
	grpcserver "github.com/Ippolid/messenger/internal/transport/grpc"
	"github.com/Ippolid/messenger/internal/transport/httpapi"

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
	httpAddr := envOr("HTTP_ADDR", ":8080")
	dsn := envOr("DB_DSN", "postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable")
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	jwtSecret := envOr("JWT_SECRET", "dev-secret-change-me")

	// Ctx отменяется по SIGINT/SIGTERM — для graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer store.Close()

	tokens := auth.NewTokenManager(jwtSecret, auth.AccessTTL)
	authSvc := auth.NewService(store, tokens)

	hub := chat.NewHub()
	chatSvc := chat.NewService(store, hub)
	limiter := ratelimit.New()

	// Брокер Redis Streams + горутины relay (outbox→Redis) и fanout (Redis→hub).
	rdb := broker.NewRedis(redisAddr, "fanout-1")
	if err := rdb.EnsureGroup(ctx); err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	go chat.NewRelay(rdb, store).Run(ctx)
	go chat.NewFanout(rdb, store, hub).Run(ctx)

	srv := grpcserver.New(authSvc, chatSvc, tokens, limiter)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              httpAddr,
		Handler:           httpapi.NewServer(authSvc, chatSvc, tokens, limiter).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Останавливаем оба сервера по сигналу.
	go func() {
		<-ctx.Done()
		log.Println("shutting down servers...")
		srv.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	// gRPC-сервер — в отдельной горутине; ошибку отдаём через канал.
	grpcErr := make(chan error, 1)
	go func() {
		log.Printf("gRPC listening on %s", addr)
		if err := srv.Serve(lis); err != nil && !errors.Is(err, net.ErrClosed) {
			grpcErr <- err
			return
		}
		grpcErr <- nil
	}()

	// HTTP-сервер — блокирующе в основной горутине.
	log.Printf("HTTP (web client) listening on %s", httpAddr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-grpcErr
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
