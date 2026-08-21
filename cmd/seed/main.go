package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/yeremi777/mlbb-analyzer-service/internal/config"
	"github.com/yeremi777/mlbb-analyzer-service/internal/staticdata"
	"github.com/yeremi777/mlbb-analyzer-service/internal/store"
)

func run() error {
	dataDir := flag.String("data", "data/static", "directory containing heroes.json")
	flag.Parse()

	_ = godotenv.Load()

	heroes, err := staticdata.LoadHeroes(*dataDir)
	if err != nil {
		return err
	}
	counters, err := staticdata.LoadCounters(*dataDir)
	if err != nil {
		return err
	}
	synergies, err := staticdata.LoadSynergies(*dataDir)
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
		if err := store.SyncHeroes(ctx, tx, heroes); err != nil {
			return err
		}
		if err := store.SyncCounters(ctx, tx, counters); err != nil {
			return err
		}
		return store.SyncSynergies(ctx, tx, synergies)
	}); err != nil {
		return err
	}

	log.Printf("synced %d heroes, %d counter matchups, %d synergy matchups",
		len(heroes), len(counters), len(synergies))
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("seed failed: %v", err)
	}
}
