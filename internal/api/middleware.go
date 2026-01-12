package api

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// Middleware — функция-обёртка для http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain применяет middleware в порядке слева направо.
// Chain(m1, m2)(handler) = m1(m2(handler))
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Logging логирует HTTP запросы.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Обёртка для захвата статуса ответа
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration", time.Since(start),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// Recovery восстанавливается после паники.
func Recovery(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
					)
					InternalError(w, logger, nil)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter — обёртка для захвата статуса ответа.
type responseWriter struct {
	http.ResponseWriter
	status int
}
