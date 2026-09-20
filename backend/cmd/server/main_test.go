package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// freePort reserves a port and releases it, so the server under test can bind something
// no other test is using.
func freePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	defer listener.Close()

	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func waitForHealth(t *testing.T, baseURL string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/healthz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server at %s did not become healthy", baseURL)
}

// The wiring test: run really listens, really serves the contract, and really stops when
// its context is cancelled.
func TestRunServesTheAPIUntilCancelled(t *testing.T) {
	port := freePort(t)
	baseURL := "http://127.0.0.1:" + port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopped := make(chan error, 1)
	go func() {
		stopped <- run(ctx, environment(map[string]string{"PORT": port, "LOG_LEVEL": "error"}), io.Discard)
	}()

	waitForHealth(t, baseURL)

	t.Run("serves the catalog from the registry", func(t *testing.T) {
		response, err := http.Get(baseURL + "/api/v1/operations")
		if err != nil {
			t.Fatalf("fetching the catalog: %v", err)
		}
		defer response.Body.Close()

		var catalog struct {
			Operations []struct {
				ID       string `json:"id"`
				Endpoint string `json:"endpoint"`
			} `json:"operations"`
		}
		if err := json.NewDecoder(response.Body).Decode(&catalog); err != nil {
			t.Fatalf("decoding the catalog: %v", err)
		}
		if len(catalog.Operations) != 7 {
			t.Errorf("catalog published %d operations, want 7", len(catalog.Operations))
		}
	})

	t.Run("computes through the whole stack", func(t *testing.T) {
		response, err := http.Post(baseURL+"/api/v1/operations/add", "application/json",
			strings.NewReader(`{"operands":[0.1,0.2]}`))
		if err != nil {
			t.Fatalf("posting: %v", err)
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("reading the response: %v", err)
		}

		want := `{"operation":"add","operands":[0.1,0.2],"result":"0.3"}`
		if strings.TrimSpace(string(body)) != want {
			t.Errorf("body = %s, want %s", strings.TrimSpace(string(body)), want)
		}
		if response.Header.Get("X-Request-Id") == "" {
			t.Error("no X-Request-Id; the middleware is not wired")
		}
	})

	cancel()

	select {
	case err := <-stopped:
		if err != nil {
			t.Errorf("run returned %v, want a clean shutdown", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return after its context was cancelled")
	}

	if _, err := http.Get(baseURL + "/healthz"); err == nil {
		t.Error("the server is still accepting connections after shutdown")
	}
}

// A misconfigured service fails at startup rather than listening on a surprising port.
func TestRunRejectsInvalidConfiguration(t *testing.T) {
	err := run(context.Background(), environment(map[string]string{"PORT": "not-a-port"}), io.Discard)

	if err == nil {
		t.Fatal("run accepted an invalid PORT")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error %q does not name the offending variable", err)
	}
}

// A port already in use is reported, not swallowed: the process must not look healthy
// while serving nothing.
func TestRunReportsAFailureToListen(t *testing.T) {
	// Occupy every interface, which is what the server binds. Holding only loopback is
	// not a conflict: Go sets SO_REUSEADDR, so a wildcard bind alongside it succeeds.
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("occupying a port: %v", err)
	}
	defer listener.Close()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	stopped := make(chan error, 1)
	go func() {
		stopped <- run(context.Background(), environment(map[string]string{"PORT": port, "LOG_LEVEL": "error"}), io.Discard)
	}()

	select {
	case err := <-stopped:
		if err == nil {
			t.Error("run returned nil for a port already in use")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not report the bind failure")
	}
}

// Logs are structured and carry the correlation id, which is what makes a request in the
// log findable from the response the client saw (ADR-0012).
func TestRequestsAreLoggedAsStructuredJSON(t *testing.T) {
	port := freePort(t)
	baseURL := "http://127.0.0.1:" + port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logs := &syncBuffer{}
	stopped := make(chan error, 1)
	go func() {
		stopped <- run(ctx, environment(map[string]string{"PORT": port}), logs)
	}()

	waitForHealth(t, baseURL)

	response, err := http.Get(baseURL + "/api/v1/operations")
	if err != nil {
		t.Fatalf("fetching the catalog: %v", err)
	}
	response.Body.Close()
	requestID := response.Header.Get("X-Request-Id")

	cancel()
	<-stopped

	var sawRequestLine bool
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line is not JSON: %v\nline: %s", err, line)
		}
		if entry["msg"] == "request" && entry["request_id"] == requestID {
			sawRequestLine = true
			if entry["path"] != "/api/v1/operations" || entry["status"] != float64(http.StatusOK) {
				t.Errorf("request log = %v, want the catalog path and status 200", entry)
			}
		}
	}

	if !sawRequestLine {
		t.Errorf("no log line correlated with %q:\n%s", requestID, logs.String())
	}
}

func environment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
