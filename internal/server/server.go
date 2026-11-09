package server

import (
	"context"
	"net/http"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/store"
)

// Server HTTP-сервер
type Server struct {
	server *http.Server
}

// NewServer создание сервера
func NewServer(port string, userStore *store.UserStore, postStore *store.PostStore, jwtManager *jwtconfig.Manager) *Server {
	mux := http.NewServeMux()

	handleConfig(mux, userStore, postStore, jwtManager)

	return &Server{
		server: &http.Server{
			Addr:    port,
			Handler: mux,
		},
	}
}

// Start запуск сервера
func (s *Server) Start() error {
	err := s.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown остановка сервера
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
