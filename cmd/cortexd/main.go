package main

import (
	"log"

	"github.com/DjarinDin/TuringSNN/internal/core_logic"
	"github.com/DjarinDin/TuringSNN/internal/service"
)

func main() {
	// Boot the full backend stack without GUI dependencies.
	backend := core_logic.NewBackend()

	// Start the headless service for remote GUI clients.
	srv := service.NewServer(backend)
	if err := srv.Start(":8080"); err != nil {
		log.Fatalf("cortexd: failed to start service: %v", err)
	}
}
