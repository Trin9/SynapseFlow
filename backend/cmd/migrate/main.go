package main

import (
	"context"
	"fmt"
	"os"

	"github.com/xunchenzheng/synapse/internal/config"
	"github.com/xunchenzheng/synapse/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/migrate <up>")
		os.Exit(1)
	}
	cfg := config.Load()
	if err := cfg.ValidateDB(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	db, err := store.OpenPostgres(context.Background(), cfg.DatabaseURL, cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxIdle, cfg.DBConnMaxLife)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	if err := store.RunMigrations(context.Background(), db, cfg.MigrationsPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
