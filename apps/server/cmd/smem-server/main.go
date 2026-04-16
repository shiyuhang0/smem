package main

import (
	"log"

	"smem/apps/server/internal/app"
	"smem/apps/server/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := application.Server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
