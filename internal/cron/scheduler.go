package cron

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Scheduler fires due cron jobs, guarding against overlapping runs of the same
// job with an in-flight set. The zero value is ready to use.
type Scheduler struct {
	inflight sync.Map // job name -> struct{}
	// run executes a fired job; nil means Job.Execute. Overridable in tests so
	// the firing logic can be exercised without spawning real subprocesses.
	run func(ctx context.Context, j Job, stardustBin, root string, w io.Writer) error
}

// fire runs a job through the injectable runner, defaulting to Job.Execute.
func (s *Scheduler) fire(ctx context.Context, j Job, stardustBin, root string, w io.Writer) error {
	if s.run != nil {
		return s.run(ctx, j, stardustBin, root, w)
	}
	return j.Execute(ctx, stardustBin, root, w)
}

// Tick fires every scheduled job whose cron expression matches now. Jobs with
// no schedule (manual or event-triggered) are skipped, as is any job whose
// previous run is still in flight (no overlap). Each fired job runs in its own
// goroutine, so Tick returns immediately; stardustBin re-execs command-kind
// jobs and root is the working directory (vault root).
func (s *Scheduler) Tick(ctx context.Context, jobs []Job, stardustBin, root string, now time.Time, w io.Writer) {
	for _, job := range jobs {
		if job.Trigger.Schedule == "" {
			continue // manual or event-triggered; the scheduler only fires schedules
		}
		match, err := Matches(job.Trigger.Schedule, now)
		if err != nil {
			fmt.Fprintf(w, "[stardust cron] bad schedule for %q: %v\n", job.Name, err)
			continue
		}
		if !match {
			continue
		}
		if _, running := s.inflight.LoadOrStore(job.Name, struct{}{}); running {
			fmt.Fprintf(w, "[stardust cron] skip %q: previous run still in flight\n", job.Name)
			continue
		}
		go func(j Job) {
			defer s.inflight.Delete(j.Name)
			if err := s.fire(ctx, j, stardustBin, root, w); err != nil {
				fmt.Fprintf(w, "[stardust cron] %q failed: %v\n", j.Name, err)
			}
		}(job)
	}
}
