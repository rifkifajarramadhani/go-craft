package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Server struct {
	server *http.Server
}

func New(port string, handler http.Handler, logger *slog.Logger) *Server {
	e := echo.New()
	e.Use(
		middleware.Recover(),
		middleware.RequestID(),
		middleware.Secure(),
		middleware.BodyLimit(64<<10),
		requestLogger(logger),
		rateLimit(20, time.Minute),
	)
	// Echo v5 Any accepts arbitrary methods. Keep the methods accepted by the v4 router.
	e.Match([]string{
		http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPatch, http.MethodPost, echo.PROPFIND, http.MethodPut, http.MethodTrace, echo.REPORT,
	}, "/*", echo.WrapHandler(handler))

	return &Server{server: &http.Server{
		Addr: ":" + port, Handler: e,
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

func requestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			started := time.Now()

			err := next(c)

			if logger != nil {
				status := 0
				if response, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil {
					status = response.Status
				}

				logger.InfoContext(c.Request().Context(), "http request", "request_id", c.Response().Header().Get(echo.HeaderXRequestID),
					"method", c.Request().Method, "path", c.Request().URL.Path,
					"status", status, "duration", time.Since(started))
			}

			return err
		}
	}
}

func rateLimit(maximum int, window time.Duration) echo.MiddlewareFunc {
	var (
		mu       sync.Mutex
		started  = time.Now()
		requests int
	)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			path := c.Request().URL.Path
			if len(path) < len("/api/auth/") || path[:len("/api/auth/")] != "/api/auth/" {
				return next(c)
			}

			mu.Lock()
			if time.Since(started) >= window {
				started = time.Now()
				requests = 0
			}

			allowed := requests < maximum
			if allowed {
				requests++
			}
			mu.Unlock()

			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
			}

			return next(c)
		}
	}
}
