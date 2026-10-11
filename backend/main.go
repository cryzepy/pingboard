package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	port := getenv("PORT", "8080")
	allowPrivate := os.Getenv("ALLOW_PRIVATE") == "true"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := NewStore(ctx, NewHTTPChecker(allowPrivate))
	if os.Getenv("SEED_DEMO") != "false" {
		seedDemo(store)
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir != "" {
		if _, err := os.Stat(staticDir); err != nil {
			log.Printf("STATIC_DIR %q tidak ditemukan, hanya melayani API", staticDir)
			staticDir = ""
		}
	}

	api := &API{store: store, staticDir: staticDir}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("PingBoard berjalan di http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server gagal: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("mematikan server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown HTTP: %v", err)
	}
	store.Shutdown()
	log.Println("selesai")
}

func seedDemo(s *Store) {
	demo := []struct{ name, url string }{
		{"Go.dev", "https://go.dev"},
		{"GitHub", "https://github.com"},
		{"Wikipedia", "https://www.wikipedia.org"},
	}
	for _, d := range demo {
		if _, err := s.Add(d.name, d.url, 30); err != nil {
			log.Printf("seed %s gagal: %v", d.name, err)
		}
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
