// Command collector ingests external feeds into the raw zone.
//
//	collector patches   fetch the patch calendar from Liquipedia
//	collector stats     fetch hero rank statistics from Moonton
//	collector all       both, patches first
//
// Re-running either feed is safe. Hero statistics are append-only, and the
// staging views resolve repeated fetches to the newest answer; the patch
// calendar is keyed on (release_date, version) and refreshed in place, so a
// re-run of an unchanged calendar writes nothing. See
// docs/specs/collector-raw-zone.md and docs/adr/0002-patch-calendar-upsert.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/yeremi777/mlbb-analyzer-service/internal/collector"
	"github.com/yeremi777/mlbb-analyzer-service/internal/config"
	"github.com/yeremi777/mlbb-analyzer-service/internal/postgres"
	"github.com/yeremi777/mlbb-analyzer-service/internal/upstream/liquipedia"
	"github.com/yeremi777/mlbb-analyzer-service/internal/upstream/moonton"
)

const (
	moontonUAEnvKey    = "COLLECTOR_USER_AGENT"
	moontonURLEnvKey   = "COLLECTOR_BASE_URL"
	liquipediaURLEnv   = "LIQUIPEDIA_BASE_URL"
	liquipediaUAEnvKey = "LIQUIPEDIA_USER_AGENT"
)

const usage = `usage: collector <command> [flags]

commands:
  patches   fetch the patch calendar from Liquipedia
  stats     fetch hero rank statistics from Moonton
  all       both, patches first

flags (stats, all):
  -date YYYY-MM-DD   override snapshot_date; default today local
  -force             collect even when the date already has rows`

// usageError marks a bad invocation: the caller gets the message and the usage
// text on stderr and exit 2, not a logged runtime failure.
type usageError struct{ error }

func badUsage(format string, a ...any) error { return usageError{fmt.Errorf(format, a...)} }

func main() {
	err := run(os.Args[1:])
	switch {
	case err == nil:
	case errors.As(err, &usageError{}):
		fmt.Fprintf(os.Stderr, "%s\n\n%s\n", err, usage)
		os.Exit(2)
	default:
		slog.Error("collector failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return badUsage("no command given")
	}
	_ = godotenv.Load()
	ctx := context.Background()

	switch cmd, rest := args[0], args[1:]; cmd {
	case "patches":
		return collectPatches(ctx)
	case "stats":
		date, force, err := statsFlags(cmd, rest)
		if err != nil {
			return err
		}
		return collectStats(ctx, date, force)
	case "all":
		date, force, err := statsFlags(cmd, rest)
		if err != nil {
			return err
		}
		// A failed patch fetch does not stop statistics: patch attribution is
		// a read-time join, so the next successful patch run repairs every
		// snapshot collected in the meantime.
		patchErr := collectPatches(ctx)
		if patchErr != nil {
			slog.Error("patch calendar failed, collecting statistics anyway", "err", patchErr)
		}
		return errors.Join(patchErr, collectStats(ctx, date, force))
	default:
		return badUsage("unknown command %q", cmd)
	}
}

func statsFlags(name string, args []string) (date string, force bool, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {} // one usage text, printed by main
	dateFlag := fs.String("date", "", "override snapshot_date (YYYY-MM-DD); default today local")
	forceFlag := fs.Bool("force", false, "collect even when the date already has rows")
	if err := fs.Parse(args); err != nil {
		return "", false, usageError{err}
	}
	if *dateFlag == "" {
		return time.Now().Format("2006-01-02"), *forceFlag, nil
	}
	if _, err := time.Parse("2006-01-02", *dateFlag); err != nil {
		return "", false, badUsage("invalid -date %q: want YYYY-MM-DD: %s", *dateFlag, err)
	}
	return *dateFlag, *forceFlag, nil
}

// connect opens the one connection a subcommand uses; the caller closes it.
func connect(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, config.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return conn, nil
}

func collectPatches(ctx context.Context) error {
	var opts []liquipedia.Option
	if base := os.Getenv(liquipediaURLEnv); base != "" {
		slog.Warn("using non-default liquipedia base URL", "base_url", base)
		opts = append(opts, liquipedia.WithBaseURL(base))
	}
	if ua := os.Getenv(liquipediaUAEnvKey); ua != "" {
		opts = append(opts, liquipedia.WithUserAgent(ua))
	}

	// Fetch before opening a transaction: a failed fetch must leave the
	// existing calendar exactly as it was.
	patches, err := liquipedia.New(opts...).FetchPatches(ctx)
	if err != nil {
		return fmt.Errorf("fetch patch calendar: %w", err)
	}
	// The parser already rejects a page with no patch rows; refusing an empty
	// calendar here too keeps the newest-patch read below unconditional.
	if len(patches) == 0 {
		return errors.New("fetch patch calendar: upstream returned no patches")
	}

	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var changed int
	if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		n, err := postgres.InsertPatches(ctx, tx, patches)
		changed = n
		return err
	}); err != nil {
		return fmt.Errorf("store patch calendar: %w", err)
	}

	slog.Info("patch calendar collected",
		"fetched", len(patches), "changed", changed, "latest", patches[0].Version,
		"released", patches[0].ReleaseDate.Format("2006-01-02"))
	return nil
}

func collectStats(ctx context.Context, date string, force bool) error {
	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var clientOpts []moonton.Option
	if base := os.Getenv(moontonURLEnvKey); base != "" {
		slog.Warn("using non-default moonton base URL", "base_url", base)
		clientOpts = append(clientOpts, moonton.WithBaseURL(base))
	}
	if ua := os.Getenv(moontonUAEnvKey); ua != "" {
		clientOpts = append(clientOpts, moonton.WithUserAgent(ua))
	}

	res, runErr := collector.Run(ctx, collector.Options{
		Date:    date,
		Force:   force,
		Store:   postgres.SnapshotStore{DB: conn},
		Fetcher: moonton.New(clientOpts...),
	})
	slog.Info("hero statistics collected", "date", res.Date, "skipped", res.Skipped,
		"combos_fetched", res.CombosFetched, "combos_failed", len(res.Failed),
		"rows_inserted", res.RowsInserted)
	return runErr
}
