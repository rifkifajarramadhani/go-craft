package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPContractHealthAndAuthorization(t *testing.T) {
	api := New(nil, nil, nil, nil, func(context.Context) bool { return true })
	handler := api.Handler()

	tests := []struct {
		name   string
		method string
		path   string
		status int
		body   string
	}{
		{name: "liveness", method: http.MethodGet, path: "/health/live", status: http.StatusOK, body: `{"status":"ok"}`},
		{name: "readiness", method: http.MethodGet, path: "/health/ready", status: http.StatusOK, body: `{"status":"ok"}`},
		{name: "protected", method: http.MethodGet, path: "/api/auth/me", status: http.StatusUnauthorized, body: `{"error":"invalid authorization header"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}

			if strings.TrimSpace(response.Body.String()) != test.body {
				t.Fatalf("body = %q, want %q", strings.TrimSpace(response.Body.String()), test.body)
			}
		})
	}
}

func TestHTTPContractRejectsMalformedAndOverspecifiedJSON(t *testing.T) {
	api := New(nil, nil, nil, nil, nil)

	tests := []string{`{"email":`, `{"email":"a@example.com","password":"secret"} {"extra":true}`}
	for _, body := range tests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		api.Handler().ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %q: status = %d", body, response.Code)
		}

		var payload map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload["error"] == "" {
			t.Fatalf("body %q: invalid error response %q", body, response.Body.String())
		}
	}
}
