package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/yeremi777/mlbb-analyzer-service/internal/config"
	"github.com/yeremi777/mlbb-analyzer-service/internal/dataset"
	"github.com/yeremi777/mlbb-analyzer-service/internal/postgres"
)

func run() error {
	dataDir := flag.String("data", "data/static", "directory containing heroes.json")
	flag.Parse()

	_ = godotenv.Load()

	ds, err := dataset.Load(*dataDir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, config.DatabaseURL())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if err := postgres.SyncHeroes(ctx, tx, ds.Heroes); err != nil {
			return err
		}
		if err := postgres.SyncCounters(ctx, tx, ds.Counters); err != nil {
			return err
		}
		return postgres.SyncSynergies(ctx, tx, ds.Synergies)
	}); err != nil {
		return err
	}

	log.Printf("synced %d heroes, %d counter matchups, %d synergy matchups",
		len(ds.Heroes), len(ds.Counters), len(ds.Synergies))
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("seed failed: %v", err)
	}
}
