package httpserver

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEchoMiddlewareAddsHeadersAndLimitsBodies(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server := New("0", handler, nil)
	response := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if response.Code != http.StatusNoContent || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status = %d, request ID = %q", response.Code, response.Header().Get("X-Request-ID"))
	}

	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(strings.Repeat("x", 64*1024+1)))
	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("body-limit status = %d, body = %s", response.Code, body)
	}
}

func TestEchoRecovery(t *testing.T) {
	server := New("0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), nil)
	response := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestEchoRoutingThrottlingAndStatusLog(t *testing.T) {
	var logs bytes.Buffer

	server := New("0", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), slog.New(slog.NewJSONHandler(&logs, nil)))

	for request := range 21 {
		response := httptest.NewRecorder()
		server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))

		want := http.StatusNoContent
		if request == 20 {
			want = http.StatusTooManyRequests
		}

		if response.Code != want {
			t.Fatalf("auth request %d status = %d, want %d", request+1, response.Code, want)
		}
	}

	if !strings.Contains(logs.String(), `"status":429`) {
		t.Fatal("request log does not contain the throttled status")
	}

	response := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusNoContent || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("root status = %d, headers = %v", response.Code, response.Header())
	}

	response = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest("CUSTOM", "/", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("custom method status = %d", response.Code)
	}
}
