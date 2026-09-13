package server

import (
	"net/http"

	"github.com/yasinatby/swappy/backend/internal/config"
)

func New(appConfig config.Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	return &http.Server{
		Addr:    appConfig.Address(),
		Handler: mux,
	}
}

func healthHandler(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(`{"status":"ok"}`))
}
