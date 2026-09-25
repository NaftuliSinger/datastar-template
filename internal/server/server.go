// Package server configures and runs the application's HTTP server.
package server

import (
	"log"
	"net"
	"net/http"

	"github.com/naftulisinger/datastar-template/internal/crypto"
	"github.com/naftulisinger/datastar-template/internal/db"
)

// Server holds whatever the handlers need, add new dependencies here
type Server struct {
	Mux    *http.ServeMux
	Addr   string
	Port   string
	DB     *db.Queries
	Crypto *crypto.Engine
}

func NewServer(addr, port string, mux *http.ServeMux, db *db.Queries, cryptoEngine *crypto.Engine) *Server {
	return &Server{
		Mux:    mux,
		Addr:   addr,
		Port:   port,
		DB:     db,
		Crypto: cryptoEngine,
	}
}

// routes use the go 1.22+ mux patterns, so methods and {id} wildcards
// work without a router library
func (s *Server) RegisterRoutes() {
	// -------- index
	// "/" also catches any path that doesn't match another route
	s.Mux.HandleFunc("GET /", s.indexPage())

	// -------- todos
	s.Mux.HandleFunc("GET /todos", s.todosPage())
	s.Mux.HandleFunc("GET /todos-search", s.todosSearch())
	s.Mux.HandleFunc("POST /todos", s.todosCreate())
	s.Mux.HandleFunc("PUT /todos/{id}/{action}", s.todosUpdate())
	s.Mux.HandleFunc("DELETE /todos/{id}", s.todosDelete())

	// -------- crypto
	s.Mux.HandleFunc("GET /crypto", s.cryptoPage())
	s.Mux.HandleFunc("GET /crypto-prices-stream", s.cryptoPrices())

	// -------- static files (favicon, images, css, ...) served from ./static
	s.Mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// -------- healthcheck
	s.Mux.HandleFunc("GET /health", handleHealthCheck)
}

func (s *Server) Start() error {
	// middleware wraps the mux so it runs before routing
	handler := logging(stripTrailingSlash(s.Mux))

	// open the port first so "ready" is only logged once it really is
	hostPort := net.JoinHostPort(s.Addr, s.Port)
	listener, err := net.Listen("tcp", hostPort)
	if err != nil {
		return err
	}
	log.Printf("server ready on http://%s", hostPort)

	return http.Serve(listener, handler)
}
