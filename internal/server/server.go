package server

import (
	"log"
	"net/http"
	"time"
)

// Server wraps the HTTP server with configuration
type Server struct {
	handler http.Handler
	port    string
}

// New creates a new HTTP server
func New(embedHandler, refreshHandler http.Handler, port, staticDir string) *Server {
	mux := http.NewServeMux()
	
	// Health endpoint (public)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	
	// Static files (public)
	staticDirHandler := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", staticDirHandler))
	
	// Report files (public - for custom JS/CSS) with CORS for development
	reportFileHandler := corsFileServer(http.Dir("./reports"))
	mux.Handle("/reports/", http.StripPrefix("/reports/", reportFileHandler))
	
	// API endpoints
	mux.Handle("/api/embed", embedHandler)
	mux.Handle("/refresh", refreshHandler)
	
	// Add middleware (logging, recovery)
	handler := addMiddleware(mux)
	
	return &Server{
		handler: handler,
		port:    port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         ":" + s.port,
		Handler:      s.handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	log.Printf("Server listening on :%s", s.port)
	return srv.ListenAndServe()
}

// corsFileServer wraps a FileServer to add CORS headers for development
func corsFileServer(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers to allow fetch() from embedded reports
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		// Handle OPTIONS preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		fileServer.ServeHTTP(w, r)
	})
}

// addMiddleware adds common middleware to the handler
func addMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log requests
		start := time.Now()
		log.Printf("%s %s", r.Method, r.URL.Path)
		
		// Add security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		
		next.ServeHTTP(w, r)
		
		log.Printf("%s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}