package httpserver

import (
	"io"
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
