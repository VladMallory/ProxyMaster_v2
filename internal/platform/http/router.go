package http

import (
	"net/http"

	"go.uber.org/zap"
)

type Server struct {
	mux        *http.ServeMux
	log        *zap.Logger
	static     string
	httpServer *http.Server
}

func NewServer(log *zap.Logger, staticDir string) *Server {
	mux := http.NewServeMux()

	s := &Server{
		mux:    mux,
		log:    log,
		static: staticDir,
	}

	// Статика для SPA
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Health check общий для всех
	mux.HandleFunc(
		"GET /health",
		func(w http.ResponseWriter, r *http.Request) {
			RespondJSON(w, http.StatusOK, "ok")
		},
	)

	handler := s.recoverMiddleware(s.loggerMiddleware(mux))

	s.httpServer = &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	return s
}

func (s *Server) Run() error {
	s.log.Info("starting http server",
		zap.String("addr", s.httpServer.Addr))

	return s.httpServer.ListenAndServe()
}

func (s *Server) Mux() *http.ServeMux {
	return s.mux
}
