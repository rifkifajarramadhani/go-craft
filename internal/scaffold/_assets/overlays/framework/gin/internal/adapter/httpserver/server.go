package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	server *http.Server
}

func New(port string, handler http.Handler, logger *slog.Logger) *Server {
	router := gin.New()
	router.Use(gin.Recovery(), requestMiddleware(logger), bodyLimit(64*1024), rateLimit(20, time.Minute))
	router.Any("/*path", gin.WrapH(handler))

	return &Server{server: &http.Server{
		Addr: ":" + port, Handler: router,
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

func requestMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Header("X-Request-ID", requestID)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")

		started := time.Now()

		c.Next()

		if logger != nil {
			logger.InfoContext(c.Request.Context(), "http request", "request_id", requestID,
				"method", c.Request.Method, "path", c.Request.URL.Path,
				"status", c.Writer.Status(), "duration", time.Since(started))
		}
	}
}

func bodyLimit(maximum int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximum)
		c.Next()
	}
}

func rateLimit(maximum int, window time.Duration) gin.HandlerFunc {
	var (
		mu       sync.Mutex
		started  = time.Now()
		requests int
	)

	return func(c *gin.Context) {
		if len(c.Request.URL.Path) < len("/api/auth/") || c.Request.URL.Path[:len("/api/auth/")] != "/api/auth/" {
			c.Next()
			return
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
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}

		c.Next()
	}
}
