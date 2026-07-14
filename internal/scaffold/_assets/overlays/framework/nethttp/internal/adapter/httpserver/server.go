package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Server struct {
	server *http.Server
}

func New(port string, handler http.Handler, logger *slog.Logger) *Server {
	limiter := newAuthLimiter(20, time.Minute)
	handler = limitBody(handler, 64*1024)
	handler = limiter.wrap(handler)
	handler = requestMiddleware(handler, logger)

	return &Server{server: &http.Server{
		Addr: ":" + port, Handler: handler,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}}
}

func (s *Server) Run(ctx context.Context) error {
	done := make(chan error, 1)

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		done <- s.server.Shutdown(shutdownCtx)
	}()

	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-done
}

func requestMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		status := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()

		defer func() {
			if recovered := recover(); recovered != nil {
				http.Error(status, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			if logger != nil {
				logger.InfoContext(r.Context(), "http request", "request_id", requestID, "method", r.Method,
					"path", r.URL.Path, "status", status.status, "duration", time.Since(started))
			}
		}()

		next.ServeHTTP(status, r)
	})
}

func limitBody(next http.Handler, bytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, bytes)
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

type authLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	started  time.Time
	requests int
}

func newAuthLimiter(maximum int, window time.Duration) *authLimiter {
	return &authLimiter{max: maximum, window: window, started: time.Now()}
}

func (l *authLimiter) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= len("/api/auth/") && r.URL.Path[:len("/api/auth/")] == "/api/auth/" && !l.allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"too many requests"}`))

			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *authLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if time.Since(l.started) >= l.window {
		l.started = time.Now()
		l.requests = 0
	}

	if l.requests >= l.max {
		return false
	}

	l.requests++

	return true
}
