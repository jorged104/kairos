package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"openscum/internal/config"
	"openscum/internal/server"
)

const startupDBTimeout = 2 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(context.Background(), logger); err != nil {
		logger.Error("la API no pudo arrancar", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := connectDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.NewRouter(logger),
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("API escuchando", slog.String("port", cfg.Port))
	return srv.ListenAndServe()
}

// connectDB opens the pool and fails if the database does not answer in time.
func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("configurar la base de datos: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, startupDBTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("la base de datos no está disponible: %w", err)
	}
	return pool, nil
}
