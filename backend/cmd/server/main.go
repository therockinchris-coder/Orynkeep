package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"orynkeep.local/backend/internal/platform"
)

func main() {
	if err := run(); err != nil {
		// Do not log database connection errors: drivers may include credentials.
		slog.Error("Orynkeep stopped", "reason", err.Error())
		os.Exit(1)
	}
}

func run() error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	storage := os.Getenv("STORAGE_PATH")
	if storage == "" {
		storage = "./data"
	}
	if err := os.MkdirAll(storage, 0700); err != nil {
		return errors.New("cannot create storage directory")
	}
	checkStorage := func() error {
		f, err := os.CreateTemp(storage, ".orynkeep-check-*")
		if err != nil {
			return err
		}
		name := f.Name()
		defer os.Remove(name)
		return f.Close()
	}
	if err := checkStorage(); err != nil {
		return errors.New("storage directory is not writable")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return errors.New("invalid database configuration")
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Initialization may briefly expose a healthy database before it restarts.
	for {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return errors.New("database unavailable after waiting 30 seconds")
		case <-time.After(time.Second):
		}
	}
	if err := platform.Migrate(ctx, db); err != nil {
		return errors.New("database migration failed; inspect schema and migration history")
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr: addr,
		Handler: platform.Handler(func(ctx context.Context) error {
			if err := db.PingContext(ctx); err != nil {
				return err
			}
			return checkStorage()
		}, &platform.SetupService{Store: platform.DatabaseSetupStore{DB: db}, Token: os.Getenv("SETUP_TOKEN")}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	finished := make(chan error, 1)
	go func() { finished <- srv.ListenAndServe() }()
	slog.Info("Orynkeep listening", "address", addr)
	select {
	case err := <-finished:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("HTTP server failed")
		}
		return nil
	case <-signals.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
