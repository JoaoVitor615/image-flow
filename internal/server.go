package internal

import (
	"net/http"

	"github.com/JoaoVitor615/image-flow/internal/controller"
	"github.com/JoaoVitor615/image-flow/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server encapsulates the HTTP router and internal API wiring
type Server struct {
	router *chi.Mux
}

// NewServer wires services, controllers, and registers HTTP routes
func NewServer() *Server {
	// Initialize domain service and HTTP controller (TODO Storage and DB)
	imageService := service.NewImageService(nil, nil)
	imageController := controller.NewImageController(imageService)

	r := chi.NewRouter()

	// Base middleware stack
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Registered endpoints
	r.Post("/images", imageController.Upload)

	return &Server{
		router: r,
	}
}

// Routes returns the configured HTTP handler for the server
func (s *Server) Routes() http.Handler {
	return s.router
}
