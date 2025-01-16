package main

import (
	"log"
	"net/http"
)

func main() {
	sm := http.NewServeMux()

	sm.HandleFunc("/healthz", healthHandler)
	sm.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir('.'))))

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
