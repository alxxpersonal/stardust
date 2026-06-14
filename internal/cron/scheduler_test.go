package cron

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectFired drives one Tick and returns the names of jobs whose runner was
// invoked, waiting briefly for the per-job goroutines to report in.
func collectFired(t *testing.T, jobs []Job, now time.Time) []string {
	t.Helper()
	fired := make(chan string, len(jobs))
	s := &Scheduler{run: func(_ context.Context, j Job, _, _ string, _ io.Writer) error {
		fired <- j.Name
		return nil
	}}
	s.Tick(context.Background(), jobs, "stardust", t.TempDir(), now, io.Discard)

	var names []string
	deadline := time.After(2 * time.Second)
	for {
		select {
		case n := <-fired:
			names = append(names, n)
		case <-time.After(150 * time.Millisecond):
			return names
		case <-deadline:
			return names
		}
	}
}

func TestSchedulerFiresOnlyDueScheduledJobs(t *testing.T) {
	jobs := []Job{
		{Name: "due", Trigger: Trigger{Schedule: "0 9 * * *"}, Run: Run{Kind: "exec", Exec: "true"}},
		{Name: "not-due", Trigger: Trigger{Schedule: "0 10 * * *"}, Run: Run{Kind: "exec", Exec: "true"}},
		{Name: "manual", Run: Run{Kind: "exec", Exec: "true"}},
		{Name: "on-event", Trigger: Trigger{On: "commit"}, Run: Run{Kind: "exec", Exec: "true"}},
	}
	fired := collectFired(t, jobs, at(9, 0))
	assert.Equal(t, []string{"due"}, fired, "only the schedule matching now fires")
}

func TestSchedulerSkipsBadSchedule(t *testing.T) {
	jobs := []Job{
		{Name: "broken", Trigger: Trigger{Schedule: "nonsense"}, Run: Run{Kind: "exec", Exec: "true"}},
		{Name: "good", Trigger: Trigger{Schedule: "* * * * *"}, Run: Run{Kind: "exec", Exec: "true"}},
	}
	fired := collectFired(t, jobs, at(9, 0))
	assert.Equal(t, []string{"good"}, fired, "a bad schedule is logged and skipped, others still fire")
}

func TestSchedulerInflightPreventsOverlap(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	s := &Scheduler{run: func(_ context.Context, j Job, _, _ string, _ io.Writer) error {
		started <- j.Name
		<-release // block so the run stays in flight across the second tick
		return nil
	}}
	job := []Job{{Name: "slow", Trigger: Trigger{Schedule: "* * * * *"}, Run: Run{Kind: "exec", Exec: "true"}}}

	s.Tick(context.Background(), job, "stardust", t.TempDir(), at(9, 0), io.Discard)
	// Wait for the first run to be in flight.
	require.Equal(t, "slow", <-started)

	// Second tick while the first is still running: must not fire again.
	s.Tick(context.Background(), job, "stardust", t.TempDir(), at(9, 1), io.Discard)
	select {
	case name := <-started:
		t.Fatalf("overlapping run fired for %q while previous was in flight", name)
	case <-time.After(250 * time.Millisecond):
		// good: no second fire
	}
	close(release)
}
