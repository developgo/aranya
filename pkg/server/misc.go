package server

import (
	"net/http"
)

func NotFoundHandler(w http.ResponseWriter, req *http.Request) {
	http.Error(w, "404 page not found", http.StatusNotFound)
}
