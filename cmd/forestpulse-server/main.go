package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"example.com/forestpulse/internal/app"
	"example.com/forestpulse/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	a, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("forestpulse listening on %s", cfg.HTTPAddr)
	if err := a.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
