package main

import (
	"encoding/json"
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

	sm.HandleFunc("GET /api/healthz", healthHandler)
	sm.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
	sm.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	sm.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
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

const metricsTemplate = `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`

func (cfg *apiConfig) metricsHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/html")
	rw.WriteHeader(200)
	rw.Write([]byte(fmt.Sprintf(metricsTemplate, cfg.fileserverHits.Load())))
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

type chirpValid struct {
	Body string `json:"body"`
}

func validateChirpHandler(rw http.ResponseWriter, req *http.Request) {
	var data chirpValid
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&data)

	if err != nil {
		rw.WriteHeader(500)
		rw.Write([]byte(`{"error":"Something went wrong"}`))
		return
	}

	if len(data.Body) > 140 {
		rw.WriteHeader(400)
		rw.Write([]byte(`{"error":"Chirp is too long"}`))
		return
	}
	rw.WriteHeader(200)
	rw.Write([]byte(`{"valid":true}`))
}
