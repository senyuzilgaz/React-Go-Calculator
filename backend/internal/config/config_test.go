package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func environment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestDefaultsApplyToAnEmptyEnvironment(t *testing.T) {
	got, err := Load(environment(nil))
	if err != nil {
		t.Fatalf("Load returned %v, want the defaults", err)
	}

	if got.Address != ":8080" {
		t.Errorf("Address = %q, want :8080", got.Address)
	}
	if got.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want INFO", got.LogLevel)
	}
	if got.AllowedOrigin != "" {
		t.Errorf("AllowedOrigin = %q, want CORS disabled by default", got.AllowedOrigin)
	}
	if got.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", got.ShutdownTimeout)
	}
}

func TestEnvironmentOverridesDefaults(t *testing.T) {
	got, err := Load(environment(map[string]string{
		"PORT":             "9090",
		"LOG_LEVEL":        "debug",
		"ALLOWED_ORIGIN":   "https://calculator.example",
		"SHUTDOWN_TIMEOUT": "30s",
	}))
	if err != nil {
		t.Fatalf("Load returned %v", err)
	}

	if got.Address != ":9090" {
		t.Errorf("Address = %q, want :9090", got.Address)
	}
	if got.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want DEBUG", got.LogLevel)
	}
	if got.AllowedOrigin != "https://calculator.example" {
		t.Errorf("AllowedOrigin = %q", got.AllowedOrigin)
	}
	if got.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", got.ShutdownTimeout)
	}
}

// A container publishing the port would be unreachable behind a loopback-only bind.
func TestAddressBindsEveryInterface(t *testing.T) {
	got, err := Load(environment(map[string]string{"PORT": "8080"}))
	if err != nil {
		t.Fatalf("Load returned %v", err)
	}
	if strings.Contains(got.Address, "localhost") || strings.Contains(got.Address, "127.0.0.1") {
		t.Errorf("Address = %q, want a bind on every interface", got.Address)
	}
}

func TestLogLevelIsCaseInsensitive(t *testing.T) {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"Warn":  slog.LevelWarn,
		"eRRoR": slog.LevelError,
	}

	for value, want := range levels {
		t.Run(value, func(t *testing.T) {
			got, err := Load(environment(map[string]string{"LOG_LEVEL": value}))
			if err != nil {
				t.Fatalf("Load returned %v", err)
			}
			if got.LogLevel != want {
				t.Errorf("LogLevel = %v, want %v", got.LogLevel, want)
			}
		})
	}
}

// A misconfigured value stops the service rather than being silently defaulted, so a typo
// surfaces at deploy time instead of as mysterious behaviour later.
func TestInvalidValuesAreRejected(t *testing.T) {
	tests := []struct {
		name        string
		environment map[string]string
		wantIn      string
	}{
		{"port is not a number", map[string]string{"PORT": "http"}, "PORT"},
		{"port is zero", map[string]string{"PORT": "0"}, "PORT"},
		{"port is negative", map[string]string{"PORT": "-1"}, "PORT"},
		{"port is out of range", map[string]string{"PORT": "65536"}, "PORT"},
		{"port has a decimal", map[string]string{"PORT": "80.80"}, "PORT"},
		{"log level is unknown", map[string]string{"LOG_LEVEL": "verbose"}, "LOG_LEVEL"},
		{"shutdown timeout is not a duration", map[string]string{"SHUTDOWN_TIMEOUT": "soon"}, "SHUTDOWN_TIMEOUT"},
		{"shutdown timeout is zero", map[string]string{"SHUTDOWN_TIMEOUT": "0s"}, "SHUTDOWN_TIMEOUT"},
		{"shutdown timeout is negative", map[string]string{"SHUTDOWN_TIMEOUT": "-5s"}, "SHUTDOWN_TIMEOUT"},
		{"shutdown timeout has no unit", map[string]string{"SHUTDOWN_TIMEOUT": "10"}, "SHUTDOWN_TIMEOUT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(environment(tt.environment))
			if err == nil {
				t.Fatalf("Load accepted %v", tt.environment)
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error %q does not name the offending variable %s", err, tt.wantIn)
			}
		})
	}
}

// Anything that could never match an Origin header is rejected at startup.
func TestAllowedOriginMustBeABareOrigin(t *testing.T) {
	rejected := []string{
		"*",
		"https://calculator.example/",
		"https://calculator.example/app",
		"calculator.example",
		"ftp://calculator.example",
		"https://",
		"https://calculator.example?a=1",
		"https://calculator.example#top",
		"https://user:pass@calculator.example",
	}

	for _, value := range rejected {
		t.Run(value, func(t *testing.T) {
			if _, err := Load(environment(map[string]string{"ALLOWED_ORIGIN": value})); err == nil {
				t.Errorf("Load accepted ALLOWED_ORIGIN %q", value)
			}
		})
	}

	accepted := []string{
		"https://calculator.example",
		"http://localhost:5173",
		"https://calculator.example:8443",
	}

	for _, value := range accepted {
		t.Run(value, func(t *testing.T) {
			got, err := Load(environment(map[string]string{"ALLOWED_ORIGIN": value}))
			if err != nil {
				t.Fatalf("Load rejected %q: %v", value, err)
			}
			if got.AllowedOrigin != value {
				t.Errorf("AllowedOrigin = %q, want %q", got.AllowedOrigin, value)
			}
		})
	}
}

// Every problem is reported at once, so one restart is enough to find them all.
func TestAllProblemsAreReportedTogether(t *testing.T) {
	_, err := Load(environment(map[string]string{
		"PORT":             "http",
		"LOG_LEVEL":        "verbose",
		"ALLOWED_ORIGIN":   "nonsense",
		"SHUTDOWN_TIMEOUT": "soon",
	}))
	if err == nil {
		t.Fatal("Load accepted four invalid values")
	}

	for _, name := range []string{"PORT", "LOG_LEVEL", "ALLOWED_ORIGIN", "SHUTDOWN_TIMEOUT"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not mention %s:\n%v", name, err)
		}
	}
}

// The middleware compares ALLOWED_ORIGIN to the Origin header byte for byte and browsers
// send it lowercased, so an origin that validates but never matches is the exact failure
// this parsing exists to prevent (ADR-0022).
func TestAllowedOriginIsNormalized(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"https://calculator.example", "https://calculator.example"},
		{"HTTPS://Calculator.Example", "https://calculator.example"},
		{"https://Calculator.Example:8443", "https://calculator.example:8443"},
		{"HTTP://LOCALHOST:5173", "http://localhost:5173"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := Load(environment(map[string]string{"ALLOWED_ORIGIN": tt.value}))
			if err != nil {
				t.Fatalf("Load returned %v", err)
			}
			if got.AllowedOrigin != tt.want {
				t.Errorf("AllowedOrigin = %q, want %q", got.AllowedOrigin, tt.want)
			}
		})
	}
}
