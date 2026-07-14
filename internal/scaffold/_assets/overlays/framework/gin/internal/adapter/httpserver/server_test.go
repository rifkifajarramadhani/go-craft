package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGinMiddlewareAddsHeadersAndLimitsBodies(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "body rejected", http.StatusRequestEntityTooLarge)

			return
		}

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
		t.Fatalf("body-limit status = %d", response.Code)
	}
}

func TestGinRecovery(t *testing.T) {
	server := New("0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), nil)
	response := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
}
