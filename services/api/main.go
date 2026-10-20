// Command api is the public Text Analysis API. It validates requests and delegates the
// analysis to the private analyzer service (specs/design.md).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/krzysztofsikora/gcp-sdd/services/api/internal/analyzer"
	"github.com/krzysztofsikora/gcp-sdd/services/api/internal/config"
	"github.com/krzysztofsikora/gcp-sdd/services/api/internal/httpapi"
)

const shutdownGrace = 10 * time.Second // NFR-03

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(ctx, os.Getenv, logger); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

// run serves until ctx is cancelled, then shuts down gracefully.
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	client := analyzer.New(cfg.AnalyzerURL, cfg.AnalyzerTimeout, nil)
	srv := &http.Server{
		Handler:           httpapi.NewHandler(client, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      cfg.AnalyzerTimeout + 10*time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		return err
	}
	logger.Info("listening", "port", cfg.Port, "analyzer_url", cfg.AnalyzerURL)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
