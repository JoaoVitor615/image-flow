package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JoaoVitor615/image-flow/internal"
	"github.com/JoaoVitor615/image-flow/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	server, err := internal.NewServer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	if err := server.Run(ctx); err != nil {
		log.Fatalf("Server terminated with error: %v", err)
	}
}
