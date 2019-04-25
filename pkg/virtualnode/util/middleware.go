/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
