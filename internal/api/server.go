package api

import (
	"net/http"

	"github.com/fernandezvara/jobqueues/internal/dashboard"
	"github.com/fernandezvara/jobqueues/internal/queue"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	router  chi.Router
	service queue.Service
}

func NewServer(service queue.Service) *Server {
	s := &Server{
		router:  chi.NewRouter(),
		service: service,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	// s.router.Use(middleware.Timeout(30))

	// API Routes
	handlers := NewHandlers(s.service)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/queues", handlers.GetQueues)
		r.Route("/queues/{name}", func(r chi.Router) {
			r.Get("/", handlers.GetQueue)
			r.Put("/", handlers.CreateOrUpdateQueue)
		})
		// r.Put("/queue/{name}", handlers.CreateOrUpdateQueue)
		r.Post("/tasks", handlers.CreateTask)
		r.Get("/tasks", handlers.GetTasks)
		r.Get("/tasks/next", handlers.GetNextTask)
		r.Route("/tasks/{id}", func(r chi.Router) {
			r.Put("/", handlers.UpdateTask)
			r.Delete("/", handlers.DeleteTask)
		})
	})

	s.router.Get("/health", handlers.HealthCheck) // Health check route

	// Dashboard routes
	filesystems := dashboard.GetFileSystem()
	fileServer := http.FileServer(http.FS(filesystems))
	s.router.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusPermanentRedirect))
	s.router.Handle("/dashboard/*", http.StripPrefix("/dashboard/", fileServer))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
