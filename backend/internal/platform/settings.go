package platform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"cashx/internal/repository"
)

// GetIntSetting reads an integer platform setting (JSON number in platform_settings).
func GetIntSetting(ctx context.Context, q *repository.Queries, key string) (int, error) {
	row, err := q.GetPlatformSetting(ctx, key)
	if err != nil {
		return 0, err
	}
	var v int
	if err := json.Unmarshal(row.Value, &v); err != nil {
		return 0, fmt.Errorf("setting %s is not an int: %w", key, err)
	}
	return v, nil
}

// GetJSONSetting reads a raw JSON platform setting value.
func GetJSONSetting(ctx context.Context, q *repository.Queries, key string) ([]byte, error) {
	row, err := q.GetPlatformSetting(ctx, key)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return row.Value, nil
}

// SetIntSetting writes an integer platform setting.
func SetIntSetting(ctx context.Context, q *repository.Queries, key string, v int) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = q.SetPlatformSetting(ctx, repository.SetPlatformSettingParams{Key: key, Value: raw})
	return err
}
