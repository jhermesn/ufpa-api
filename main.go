package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	defaultAddr = ":8080"
	// Must stay below the ECS StopTimeout (10s) so the task exits before SIGKILL.
	shutdownTimeout = 8 * time.Second
	// Must exceed the ALB idle timeout (60s) to avoid 502s on reused connections.
	idleTimeout = 65 * time.Second
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	info := loadTaskInfo(ctx, os.Getenv("ECS_CONTAINER_METADATA_URI_V4"))
	srv := newServer(listenAddr(os.Getenv("PORT")), newHandler(info))

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("server started", "addr", srv.Addr, "version", version, "task_id", info.TaskID, "az", info.AZ)

	select {
	case err := <-errCh:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
		return shutdown(srv)
	}
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       idleTimeout,
	}
}

func listenAddr(port string) string {
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return defaultAddr
	}
	return ":" + port
}

func shutdown(srv *http.Server) error {
	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
