package repository

import (
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Helpers to convert between nullable pgtype columns and plain Go values.

// UUIDPtr converts a *string (nil = NULL) to pgtype.UUID.
func UUIDPtr(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(*s)
	return u
}

// UUIDToPtr converts pgtype.UUID to *string.
func UUIDToPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

// TextPtr converts a *string (nil = NULL) to pgtype.Text.
func TextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// TextToPtr converts pgtype.Text to *string.
func TextToPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// TimePtr converts a *time.Time (nil = NULL) to pgtype.Timestamptz.
func TimePtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// TimeToPtr converts pgtype.Timestamptz to *time.Time.
func TimeToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// DatePtr converts a time.Time to pgtype.Date.
func DatePtr(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// Float64ToNum converts float64 to pgtype.Numeric (nullable aware).
func Float64ToNum(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(*f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// NumToFloat64 converts pgtype.Numeric to float64.
func NumToFloat64(n pgtype.Numeric) float64 {
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return v.Float64
}

// BoolPtr converts a *bool (nil = NULL) to pgtype.Bool.
func BoolPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// Int64ToPg converts a *int64 (nil = NULL) to pgtype.Int8.
func Int64ToPg(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

// Int64ToPtr converts pgtype.Int8 to *int64.
func Int64ToPtr(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}

// Float64ToPtr converts pgtype.Float8 to *float64.
func Float64ToPtr(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// Int32Ptr converts a *int (nil = NULL) to pgtype.Int4.
func Int32Ptr(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}
