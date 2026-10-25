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

	server := internal.NewServer()

	log.Printf("API server running on port %s", port)
	if err := http.ListenAndServe(":"+port, server.Routes()); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
