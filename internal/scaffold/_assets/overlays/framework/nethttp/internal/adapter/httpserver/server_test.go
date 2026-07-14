package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNativeMiddlewareAddsRequestMetadataAndSecurityHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server := New("0", handler, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}

	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}

	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing security headers")
	}
}

func TestNativeMiddlewareLimitsBodiesAndRecoversPanics(t *testing.T) {
	readHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "body rejected", http.StatusRequestEntityTooLarge)
		}
	})
	server := New("0", readHandler, nil)
	request := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(strings.Repeat("x", 64*1024+1)))
	response := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body-limit status = %d", response.Code)
	}

	panicServer := New("0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), nil)
	response = httptest.NewRecorder()
	panicServer.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("recovery status = %d", response.Code)
	}
}
