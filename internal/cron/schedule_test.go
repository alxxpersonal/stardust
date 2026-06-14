package cron

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func at(h, m int) time.Time {
	return time.Date(2026, 6, 14, h, m, 0, 0, time.UTC)
}

func TestMatchesDailyAtNine(t *testing.T) {
	for _, tc := range []struct {
		t    time.Time
		want bool
	}{
		{at(9, 0), true},
		{at(9, 1), false},
		{at(8, 59), false},
		{at(21, 0), false},
	} {
		got, err := Matches("0 9 * * *", tc.t)
		require.NoError(t, err)
		assert.Equalf(t, tc.want, got, "0 9 * * * at %s", tc.t.Format("15:04"))
	}
}

func TestMatchesEveryFiveMinutes(t *testing.T) {
	got, err := Matches("*/5 * * * *", at(9, 5))
	require.NoError(t, err)
	assert.True(t, got, "09:05 is a */5 boundary")

	got, err = Matches("*/5 * * * *", at(9, 6))
	require.NoError(t, err)
	assert.False(t, got, "09:06 is not a */5 boundary")

	got, err = Matches("*/5 * * * *", at(9, 0))
	require.NoError(t, err)
	assert.True(t, got, "09:00 is a */5 boundary")
}

func TestMatchesEveryMinute(t *testing.T) {
	got, err := Matches("* * * * *", at(13, 37))
	require.NoError(t, err)
	assert.True(t, got)
}

func TestMatchesIgnoresSubMinute(t *testing.T) {
	// A non-zero second within a matching minute still matches (minute truncation).
	got, err := Matches("0 9 * * *", time.Date(2026, 6, 14, 9, 0, 45, 0, time.UTC))
	require.NoError(t, err)
	assert.True(t, got)
}

func TestMatchesInvalidExpr(t *testing.T) {
	_, err := Matches("not a cron expr", at(9, 0))
	assert.Error(t, err)
}
