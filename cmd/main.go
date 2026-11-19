package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/DillonEnge/slopify/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "7600"
	}

	addr := ":" + port
	slog.Info("Listening on " + addr)
	return service.NewServer().ListenAndServe(addr)
}
