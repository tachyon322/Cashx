package tracking

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/platform"
	"cashx/internal/repository"
)

// ClickResult is the outcome of recording a click.
type ClickResult struct {
	ClickID     int64
	Destination string
}

// EnsurePartitionsFor creates the monthly partitions of the tracking tables
// for the month of ts plus the current and next month (SECURITY DEFINER
// helper from migrations/00021_partition_security.sql). Safe to call from
// the app role.
func EnsurePartitionsFor(ctx context.Context, pool *pgxpool.Pool, ts time.Time) error {
	_, err := pool.Exec(ctx, "SELECT ensure_partitions_for($1)", ts)
	return err
}

// isNoPartitionErr reports the "no partition of relation ... found for row"
// error (SQLSTATE 23514) — the 2026-09-01 ingestion outage shape.
func isNoPartitionErr(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23514"
}

// insertWithPartitionRetry runs insert once; on 23514 it ensures the
// partitions for the row's month and retries once. No trigger can create a
// partition for the table being inserted into (row triggers fire after
// routing, statement triggers self-lock), so the retry lives in Go.
func InsertWithPartitionRetry(ctx context.Context, pool *pgxpool.Pool, ts time.Time, insert func() error) error {
	err := insert()
	if err == nil || !isNoPartitionErr(err) {
		return err
	}
	if ensureErr := EnsurePartitionsFor(ctx, pool, ts); ensureErr != nil {
		return err // original 23514 is the more actionable error
	}
	return insert()
}

// RecordClick resolves a tracking link code, records the click and returns the
// destination URL for the redirect.
func RecordClick(ctx context.Context, pool *pgxpool.Pool, code, ip, userAgent, referrer string) (ClickResult, error) {
	q := repository.New(pool)
	link, err := q.GetTrackingLinkByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ClickResult{}, fmt.Errorf("%w: link_not_found", platform.ErrNotFound)
		}
		return ClickResult{}, err
	}
	if !link.IsActive || link.AccessStatus != "active" {
		return ClickResult{}, fmt.Errorf("%w: link_not_found", platform.ErrNotFound)
	}
	// Promo codes are not redirectable via /c/
	if link.Type == "promo" {
		return ClickResult{}, fmt.Errorf("%w: link_not_found", platform.ErrNotFound)
	}
	params := repository.CreateTrackingClickParams{
		TrackingLinkID: link.ID,
		Column2:        ip,
		UserAgent:      repository.TextPtr(&userAgent),
		Referrer:       repository.TextPtr(&referrer),
	}
	var click repository.TrackingClick
	err = InsertWithPartitionRetry(ctx, pool, time.Now(), func() error {
		var e error
		click, e = q.CreateTrackingClick(ctx, params)
		return e
	})
	if err != nil {
		return ClickResult{}, err
	}
	// Counters (best-effort)
	if DefaultCounters != nil {
		_ = DefaultCounters.RecordClick(ctx, link.ID, ClickMeta{IP: ip, UserAgent: userAgent, Referrer: referrer})
	}
	dest := ""
	// Weighted redirect pool takes precedence
	if link.RedirectID.Valid {
		urls, err := q.ListRedirectPoolURLs(ctx, link.RedirectID.String())
		if err == nil && len(urls) > 0 {
			dest = weightedPick(urls)
		}
	}
	// Domain chosen on the source (or its mirror): the player lands on the
	// mirror itself — but only while it is still an active domain of the offer.
	if dest == "" && link.Domain.Valid && link.Domain.String != "" && link.DomainActive.Valid && link.DomainActive.Bool {
		dest = link.Domain.String
	}
	if dest == "" && link.OfferDestinationUrl.Valid {
		dest = link.OfferDestinationUrl.String
	}
	if dest == "" {
		dest = link.ProjectDestinationUrl
	}
	return ClickResult{ClickID: click.ID, Destination: dest}, nil
}

func weightedPick(urls []repository.RedirectPoolUrl) string {
	active := make([]repository.RedirectPoolUrl, 0, len(urls))
	for _, u := range urls {
		if u.IsActive && u.Url != "" {
			active = append(active, u)
		}
	}
	if len(active) == 0 {
		return ""
	}
	total := 0
	for _, u := range active {
		w := int(u.Weight)
		if w < 1 {
			w = 1
		}
		total += w
	}
	r := rand.Intn(total)
	for _, u := range active {
		w := int(u.Weight)
		if w < 1 {
			w = 1
		}
		r -= w
		if r < 0 {
			return u.Url
		}
	}
	return active[0].Url
}
