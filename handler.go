package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const appName = "ufpa-api"

var version = "dev"

type infoResponse struct {
	App     string `json:"app"`
	Version string `json:"version"`
	TaskID  string `json:"task_id"`
	AZ      string `json:"az"`
}

func newHandler(info TaskInfo) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/info", infoHandler(info))
	mux.HandleFunc("GET /healthz", healthHandler)
	return withRequestLog(mux)
}

func infoHandler(info TaskInfo) http.HandlerFunc {
	body := infoResponse{App: appName, Version: version, TaskID: info.TaskID, AZ: info.AZ}
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, body)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path == "/healthz" {
			return
		}
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}
