package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func Handler(check func(context.Context) error, setup ...*SetupService) http.Handler {
	mux := http.NewServeMux()
	if len(setup) > 0 && setup[0] != nil {
		setup[0].register(mux)
	}
	mux.HandleFunc("GET /api/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := check(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"name": "Orynkeep", "version": "0.1.0-dev", "stage": "administrator-setup"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
