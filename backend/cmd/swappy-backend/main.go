package main

import (
	"log"

	"github.com/yasinatby/swappy/backend/internal/config"
	"github.com/yasinatby/swappy/backend/internal/server"
)

func main() {
	appConfig := config.Load()
	httpServer := server.New(appConfig)

	log.Printf("swappy backend listening on %s", appConfig.Address())
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
