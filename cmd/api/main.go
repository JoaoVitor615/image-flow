package main

import (
	"log"
	"net/http"
	"os"

	"github.com/JoaoVitor615/image-flow/internal"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := internal.SetupRouter()

	log.Printf("API server running on port %s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
