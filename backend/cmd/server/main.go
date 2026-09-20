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
	transport "github.com/ilgazsenyuz/sezzle-technical-assignment/backend/internal/transport/http"
)

// Deadlines for the connection itself, as opposed to anything the handlers do. They are
// constants rather than settings because no deployment of this service has a reason to
// differ: the API is pure computation with a 4 KiB body limit, so any request that takes
// this long is a slow or stuck client rather than slow work.
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

// run is main's body, taking its environment as arguments so the wiring can be exercised
// by a test rather than only by starting the process.
func run(ctx context.Context, getenv func(string) string, logOutput io.Writer) error {
	settings, err := config.Load(getenv)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(logOutput, &slog.HandlerOptions{Level: settings.LogLevel}))

	server := &http.Server{
		Addr: settings.Address,
		Handler: transport.NewRouter(transport.Config{
			AllowedOrigin: settings.AllowedOrigin,
			Logger:        logger,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,

		// net/http logs its own faults through a log.Logger; routing it into slog keeps
		// every line the process emits in one structured stream.
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

	// Stop intercepting signals before waiting: a second Ctrl-C should terminate the
	// process immediately rather than sit out the grace period.
	stop()
	logger.Info("shutting down", "grace", settings.ShutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settings.ShutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
