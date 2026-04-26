package api

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/options", optionsHandler)
	mux.HandleFunc("/api/v1/generate", generateHandler)
	mux.HandleFunc("/api/v1/requests/", requestHandler)          // expects id suffix
	mux.HandleFunc("/api/v1/internal/callback/", callbackHandler) // Lambda async callback
	mux.HandleFunc("/health", healthHandler)
}
