package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	apiCfg := apiConfig{}

	sm := http.NewServeMux()

	sm.HandleFunc("GET /healthz", healthHandler)
	sm.HandleFunc("GET /metrics", apiCfg.metricsHandler)
	sm.HandleFunc("POST /reset", apiCfg.resetHandler)
	sm.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir('.')))))

	server := http.Server{
		Addr:    ":8080",
		Handler: sm,
	}
	log.Fatal(server.ListenAndServe())
}

func healthHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(200)
	rw.Write([]byte("OK"))
}

func (cfg *apiConfig) metricsHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(200)
	rw.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) resetHandler(rw http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	rw.WriteHeader(200)
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
