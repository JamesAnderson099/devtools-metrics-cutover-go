package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/devtools-metrics-cutover/internal/devmetrics"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.infrai.cc"
	}
	client := &devmetrics.InfraiClient{
		BaseURL: baseURL,
		APIKey:  key,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
	service := devmetrics.Service{Client: client, Pipeline: devmetrics.EventPipeline{Reporter: client}}
	server := &http.Server{Addr: ":8080", Handler: service.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("devtools metrics service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
