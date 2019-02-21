package util

import (
	"github.com/go-logr/logr"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func PanicRecoverMiddleware(logger logr.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				err := recover()
				if err != nil {
					logger.V(2).Info("Recovered from panic", "Panic", err)
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}

func LogMiddleware(logger logr.Logger) mux.MiddlewareFunc {
	log := logger.WithName("request")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			log.Info("request started", "Request.URI", req.URL.RequestURI(), "Request.RemoteAddr", req.RemoteAddr)
			startTime := time.Now()
			defer func() {
				log.Info("request finished", "Request.Duration", time.Since(startTime).String())
			}()

			next.ServeHTTP(w, req)
		})
	}
}

func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "404 page not found", http.StatusNotFound)
	})
}
