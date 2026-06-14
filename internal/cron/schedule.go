package cron

import (
	"time"

	"github.com/robfig/cron/v3"
)

// Matches reports whether the standard 5-field cron expression expr fires at
// time t, to minute resolution. expr is interpreted in t's location. The next
// activation strictly after (t's minute - 1s) equals that minute iff the minute
// is itself a scheduled fire time.
func Matches(expr string, t time.Time) (bool, error) {
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return false, err
	}
	minute := t.Truncate(time.Minute)
	return sched.Next(minute.Add(-time.Second)).Equal(minute), nil
}
