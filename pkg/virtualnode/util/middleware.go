package util

import (
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
)

func PanicRecoverMiddleware(logger logr.Logger) mux.MiddlewareFunc {
	log := logger
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				err := recover()
				if err != nil {
					log.Info("recovered from panic", "panic", err)
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}

func LogMiddleware(logger logr.Logger) mux.MiddlewareFunc {
	log := logger.WithName("request.log")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			log.Info("request started", "uri", req.URL.RequestURI(), "remote", req.RemoteAddr)
			startTime := time.Now()
			defer func() {
				log.Info("request finished", "dur", time.Since(startTime).String())
			}()

			next.ServeHTTP(w, req)
		})
	}
}

func NotFoundHandler(logger logr.Logger) http.Handler {
	log := logger.WithName("request.notFound").V(10)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("request url not handled", "uri", r.URL.RequestURI())
		http.Error(w, "404 page not found", http.StatusNotFound)
	})
}
