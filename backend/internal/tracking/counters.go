package tracking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Counters is a Redis-backed buffer for click/signup stats, mirroring kazik's affiliateCounters.
type Counters struct {
	redis *redis.Client
}

func NewCounters(r *redis.Client) *Counters {
	return &Counters{redis: r}
}

// ClickMeta holds click attribution meta.
type ClickMeta struct {
	IP        string
	UserAgent string
	Referrer  string
}

func (c *Counters) keyStats(linkID string) string { return "cashx:stats:" + linkID }
func (c *Counters) keyUniq(linkID string) string  { return "cashx:uniq:" + linkID }
func (c *Counters) keyClicksBuf() string          { return "cashx:clicks_buf" }
func (c *Counters) keySeeded(linkID string) string { return "cashx:seeded:" + linkID }

// RecordClick increments Redis counters and buffers the click for batch PG insert.
func (c *Counters) RecordClick(ctx context.Context, linkID string, meta ClickMeta) error {
	if c.redis == nil {
		return nil
	}
	pipe := c.redis.Pipeline()
	pipe.HIncrBy(ctx, c.keyStats(linkID), "clicks", 1)
	if meta.IP != "" {
		pipe.PFAdd(ctx, c.keyUniq(linkID), meta.IP)
	}
	// Buffer for flush
	buf, _ := json.Marshal(map[string]string{
		"link_id":    linkID,
		"ip":         meta.IP,
		"user_agent": meta.UserAgent,
		"referrer":   meta.Referrer,
		"created_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	pipe.LPush(ctx, c.keyClicksBuf(), string(buf))
	pipe.Expire(ctx, c.keyStats(linkID), 7*24*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		// Log but don't fail caller
		fmt.Printf("[counters] RecordClick err: %v\n", err)
	}
	return nil
}

// RecordSignup increments signup counters.
func (c *Counters) RecordSignup(ctx context.Context, linkID, userID, kind string) error {
	if c.redis == nil {
		return nil
	}
	field := "signups"
	if kind == "promo" {
		field = "promos"
	}
	// Use SADD for dedup? For now simple HIncrBy + SADD set
	setKey := fmt.Sprintf("cashx:signups:%s:%s", linkID, kind)
	pipe := c.redis.Pipeline()
	added := pipe.SAdd(ctx, setKey, userID)
	pipe.Expire(ctx, setKey, 30*24*time.Hour)
	if _, err := pipe.Exec(ctx); err == nil {
		if n, _ := added.Result(); n > 0 {
			c.redis.HIncrBy(ctx, c.keyStats(linkID), field, 1)
		}
	}
	return nil
}

// GetStats returns aggregated stats from Redis if available, else nil to indicate fallback.
func (c *Counters) GetStats(ctx context.Context, linkIDs []string) (map[string]StatsAggregate, bool) {
	if c.redis == nil || len(linkIDs) == 0 {
		return nil, false
	}
	m := make(map[string]StatsAggregate, len(linkIDs))
	for _, id := range linkIDs {
		vals, err := c.redis.HGetAll(ctx, c.keyStats(id)).Result()
		if err != nil || len(vals) == 0 {
			return nil, false
		}
		// Check seeded flag? For now require stats exists
		if _, ok := vals["clicks"]; !ok {
			return nil, false
		}
		// Parse ints
		agg := StatsAggregate{}
		agg.Clicks = parseInt(vals["clicks"])
		agg.UniqueClicks = c.redis.PFCount(ctx, c.keyUniq(id)).Val()
		agg.Signups = parseInt(vals["signups"])
		agg.Promos = parseInt(vals["promos"])
		// Deposits/income are stored via separate increments from conversion worker; for now 0
		agg.DepositsSum = parseInt(vals["deposits_sum"])
		agg.Income = parseInt(vals["income"])
		m[id] = agg
	}
	return m, true
}

func parseInt(s string) int64 {
	var v int64
	fmt.Sscan(s, &v)
	return v
}

// Flush drains the clicks buffer and inserts into PG (batch 500). Caller should run in worker.
func (c *Counters) Flush(ctx context.Context, insertFn func(ctx context.Context, linkID, ip, ua, ref string, at time.Time) error) error {
	if c.redis == nil {
		return nil
	}
	for {
		// Pop batch
		vals, err := c.redis.BRPop(ctx, 0, c.keyClicksBuf()).Result()
		if err != nil {
			if err == redis.Nil {
				return nil
			}
			return err
		}
		if len(vals) < 2 {
			continue
		}
		data := vals[1]
		var evt map[string]string
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, evt["created_at"])
		if at.IsZero() {
			at = time.Now().UTC()
		}
		if err := insertFn(ctx, evt["link_id"], evt["ip"], evt["user_agent"], evt["referrer"], at); err != nil {
			fmt.Printf("[counters] flush insert err: %v\n", err)
		}
		// Try to drain more without blocking
		for i := 0; i < 500; i++ {
			v, err := c.redis.RPop(ctx, c.keyClicksBuf()).Result()
			if err != nil {
				break
			}
			var e2 map[string]string
			if err := json.Unmarshal([]byte(v), &e2); err != nil {
				continue
			}
			at2, _ := time.Parse(time.RFC3339Nano, e2["created_at"])
			if at2.IsZero() {
				at2 = time.Now().UTC()
			}
			_ = insertFn(ctx, e2["link_id"], e2["ip"], e2["user_agent"], e2["referrer"], at2)
		}
		return nil
	}
}

// Global singleton (set in main/worker).
var DefaultCounters *Counters
