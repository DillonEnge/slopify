package service

import (
	"io/fs"
	"net/http"

	handle "github.com/DillonEnge/http-handler-go"
	"github.com/DillonEnge/middleware-go"
	v1 "github.com/DillonEnge/slopify/internal/api/v1"
	"github.com/DillonEnge/slopify/internal/site"
	"github.com/DillonEnge/slopify/internal/template"
	"github.com/DillonEnge/slopify/internal/textgen"
)

type Server struct {
	static fs.FS
	pages  *site.Store
	gen    site.Generator
}

func NewServer() *Server {
	return newServer(textgen.NewClient())
}

func newServer(gen site.Generator) *Server {
	static, err := fs.Sub(template.Static, "static")
	if err != nil {
		panic(err)
	}
	return &Server{
		static: static,
		pages:  site.NewStore(),
		gen:    gen,
	}
}

func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()

	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	m.Handle(
		"GET /static/",
		middleware.NoCache(http.StripPrefix("/static/", http.FileServer(http.FS(s.static)))),
	)

	m.HandleFunc("GET /new-site", handle.NewHandlerFunc(v1.HandleNewSite(s.pages)))
	state := handle.NewHandlerFunc(v1.HandleState(s.pages, s.gen))
	m.HandleFunc("GET /state/{path...}", state)
	m.HandleFunc("POST /state/{path...}", state)
	m.HandleFunc("PUT /state/{path...}", state)
	m.HandleFunc("PATCH /state/{path...}", state)
	m.HandleFunc("DELETE /state/{path...}", state)
	m.HandleFunc("GET /{$}", handle.NewHandlerFunc(v1.HandlePage(s.pages, s.gen)))
	m.HandleFunc("GET /{path...}", handle.NewHandlerFunc(v1.HandlePage(s.pages, s.gen)))

	return m
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}
