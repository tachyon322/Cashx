// backfill-stats: one-shot recomputation of daily_partner_offer_stats and
// daily_tracking_link_stats over the whole history (or a -from/-to range).
//
// Run it after a kazik ETL pass: the ETL inserts raw rows (clicks,
// attributions, conversions, earnings) with their original timestamps, but
// the worker only ever recomputes the last 3 MSK days — without this
// backfill the cabinet charts show migrated traffic lumped into a couple of
// days and nothing before that.
//
// Chunked month-by-month in ascending order: each pass deletes aggregate
// rows from its own month start onward and rewrites that month, so later
// passes never destroy earlier results (see tracking.RecomputeDailyStats).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/platform"
	"cashx/internal/tracking"
)

func main() {
	fromFlag := flag.String("from", "", "inclusive start (2006-01-02), default: earliest data")
	toFlag := flag.String("to", "", "exclusive end (2006-01-02), default: tomorrow")
	flag.Parse()

	cfg, err := platform.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	log := platform.NewLogger(cfg.Env)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := platform.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	var from, to time.Time
	if *fromFlag != "" {
		from, err = time.Parse("2006-01-02", *fromFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "-from: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Earliest timestamp across the raw tables that feed the aggregates.
		var minClick, minAttr, minConv, minEarn *time.Time
		if v, err := minTS(ctx, db, "SELECT min(created_at) FROM tracking_clicks"); err == nil {
			minClick = v
		}
		if v, err := minTS(ctx, db, "SELECT min(first_seen_at) FROM external_user_attributions"); err == nil {
			minAttr = v
		}
		if v, err := minTS(ctx, db, "SELECT min(occurred_at) FROM conversion_events"); err == nil {
			minConv = v
		}
		if v, err := minTS(ctx, db, "SELECT min(created_at) FROM commission_earnings"); err == nil {
			minEarn = v
		}
		from = earliest(minClick, minAttr, minConv, minEarn)
		if from.IsZero() {
			log.Info("no data to backfill")
			return
		}
	}
	if *toFlag != "" {
		to, err = time.Parse("2006-01-02", *toFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "-to: %v\n", err)
			os.Exit(1)
		}
	} else {
		to = time.Now().AddDate(0, 0, 1)
	}
	log.Info("backfilling daily stats", "from", from.Format("2006-01-02"), "to", to.Format("2006-01-02"))

	months := 0
	for m := from; m.Before(to); m = m.AddDate(0, 1, 0) {
		end := m.AddDate(0, 1, 0)
		if end.After(to) {
			end = to
		}
		if err := tracking.RecomputeDailyStats(ctx, db, m, end); err != nil {
			log.Error("backfill month failed", "month", m.Format("2006-01"), "err", err)
			os.Exit(1)
		}
		months++
		log.Info("backfilled", "month", m.Format("2006-01"))
		if err := ctx.Err(); err != nil {
			log.Warn("interrupted", "err", err)
			os.Exit(1)
		}
	}
	log.Info("backfill done", "months", months)
}

func minTS(ctx context.Context, db *pgxpool.Pool, q string) (*time.Time, error) {
	var v *time.Time
	if err := db.QueryRow(ctx, q).Scan(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func earliest(ts ...*time.Time) time.Time {
	var out time.Time
	for _, t := range ts {
		if t == nil {
			continue
		}
		if out.IsZero() || t.Before(out) {
			out = *t
		}
	}
	return out
}
