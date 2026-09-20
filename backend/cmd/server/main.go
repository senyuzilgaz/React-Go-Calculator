// Command server runs the calculator API.
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilgazsenyuz/sezzle-technical-assignment/backend/internal/config"
	"github.com/ilgazsenyuz/sezzle-technical-assignment/backend/internal/transport/httpapi"
)

// Connection deadlines, not handler budgets. Constants rather than settings: the API is pure
// computation behind a 4 KiB cap, so a request this slow is a stuck client.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	if err := run(context.Background(), os.Getenv, os.Stdout); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// main's body, taking its environment as arguments so a test can drive the wiring.
func run(ctx context.Context, getenv func(string) string, logOutput io.Writer) error {
	settings, err := config.Load(getenv)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(logOutput, &slog.HandlerOptions{Level: settings.LogLevel}))

	server := &http.Server{
		Addr: settings.Address,
		Handler: httpapi.NewRouter(httpapi.Config{
			AllowedOrigin: settings.AllowedOrigin,
			Logger:        logger,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,

		// Keeps net/http's own log.Logger in the same structured stream.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	serving := make(chan error, 1)
	go func() {
		logger.Info("listening",
			"address", server.Addr,
			"log_level", settings.LogLevel.String(),
			"cors_enabled", settings.AllowedOrigin != "",
		)
		serving <- server.ListenAndServe()
	}()

	select {
	case err := <-serving:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err

	case <-ctx.Done():
	}

	// Stop intercepting first, so a second Ctrl-C terminates instead of waiting out the
	// grace period.
	stop()
	logger.Info("shutting down", "grace", settings.ShutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settings.ShutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
