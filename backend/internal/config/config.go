// Package config reads the service's settings from the environment.
//
// Load is a function of the lookup it is given rather than of os.Getenv directly, so
// configuration can be exercised without touching process state.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort            = 8080
	defaultLogLevel        = slog.LevelInfo
	defaultShutdownTimeout = 10 * time.Second

	maxPort = 65535
)

type Config struct {
	// Address is the listen address, always on every interface: the service runs in a
	// container and a loopback-only bind would be unreachable from outside it (ADR-0012).
	Address string

	LogLevel slog.Level

	// AllowedOrigin enables CORS for exactly that origin. Empty disables it, which is the
	// development default, where the Vite dev server proxies the API (ADR-0012).
	AllowedOrigin string

	// ShutdownTimeout is how long in-flight requests have to finish once a termination
	// signal arrives.
	ShutdownTimeout time.Duration
}

// Load reads the configuration, reporting every problem it finds rather than only the
// first: a misconfigured deployment should need one restart to diagnose, not four.
func Load(getenv func(string) string) (Config, error) {
	address, addressErr := parseAddress(getenv("PORT"))
	logLevel, logLevelErr := parseLogLevel(getenv("LOG_LEVEL"))
	allowedOrigin, allowedOriginErr := parseAllowedOrigin(getenv("ALLOWED_ORIGIN"))
	shutdownTimeout, shutdownTimeoutErr := parseShutdownTimeout(getenv("SHUTDOWN_TIMEOUT"))

	if err := errors.Join(addressErr, logLevelErr, allowedOriginErr, shutdownTimeoutErr); err != nil {
		return Config{}, err
	}

	return Config{
		Address:         address,
		LogLevel:        logLevel,
		AllowedOrigin:   allowedOrigin,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

func parseAddress(value string) (string, error) {
	if value == "" {
		return fmt.Sprintf(":%d", defaultPort), nil
	}

	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > maxPort {
		return "", fmt.Errorf("PORT %q is not a port number between 1 and %d", value, maxPort)
	}
	return fmt.Sprintf(":%d", port), nil
}

func parseLogLevel(value string) (slog.Level, error) {
	if value == "" {
		return defaultLogLevel, nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(value))); err != nil {
		return 0, fmt.Errorf("LOG_LEVEL %q is not one of debug, info, warn, error", value)
	}
	return level, nil
}

// parseAllowedOrigin insists on a bare origin, because the CORS middleware compares it to
// the Origin header exactly. A trailing slash or a path would parse, deploy, and then
// silently match nothing.
func parseAllowedOrigin(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if value == "*" {
		return "", errors.New(`ALLOWED_ORIGIN "*" is not supported; name a single origin such as https://calculator.example`)
	}

	origin, err := url.Parse(value)
	if err != nil ||
		(origin.Scheme != "http" && origin.Scheme != "https") ||
		origin.Host == "" ||
		origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return "", fmt.Errorf("ALLOWED_ORIGIN %q is not a bare origin such as https://calculator.example", value)
	}
	return value, nil
}

func parseShutdownTimeout(value string) (time.Duration, error) {
	if value == "" {
		return defaultShutdownTimeout, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("SHUTDOWN_TIMEOUT %q is not a positive duration such as 10s", value)
	}
	return timeout, nil
}
