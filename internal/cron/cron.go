// Package cron is the declarative scheduler. Each job is a folder under
// .stardust/cron-jobs/ with a config.toml describing a trigger and one of three
// run kinds: a stardust command, an external exec, or an agent. v1 loads and
// runs jobs on demand; a launchd or cron timer drives them by calling cron run.
package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// Trigger declares when a job should fire. v1 records it for display; the OS
// timer (or commit hook) is what actually invokes `cron run`.
type Trigger struct {
	Schedule string   `toml:"schedule"` // cron expression, e.g. "0 3 * * *"
	On       string   `toml:"on"`       // event, e.g. "commit"
	Paths    []string `toml:"paths"`    // path globs gating an "on" trigger
}

// DefaultCodexModel is used when a codex agent job names no model. The built-in
// codex default (gpt-5.3-codex) is rejected on ChatGPT-auth accounts, so pin a
// supported one. Mirrors exo-jobs.
const DefaultCodexModel = "gpt-5.5"

// Run declares what a job does. Exactly one kind applies.
type Run struct {
	Kind    string `toml:"kind"`             // agent | command | exec
	Command string `toml:"command"`          // a stardust subcommand (kind=command)
	Exec    string `toml:"exec"`             // an external shell command (kind=exec)
	Prompt  string `toml:"prompt"`           // prompt file relative to the job dir (kind=agent)
	Model   string `toml:"model"`            // agent model (kind=agent)
	Sandbox string `toml:"sandbox"`          // agent sandbox mode (kind=agent)
	Runner  string `toml:"runner,omitempty"` // agent backend: codex | claude (kind=agent)
}

// ResolvedRunner returns the agent backend for a kind=agent job: the explicit
// Runner when set, else the legacy default (codex) so pre-runner job files keep
// running as before.
func (j Job) ResolvedRunner() string {
	if j.Run.Runner != "" {
		return j.Run.Runner
	}
	return "codex"
}

// Job is one loaded cron-job folder.
type Job struct {
	Name    string  `toml:"-"`
	Dir     string  `toml:"-"`
	Trigger Trigger `toml:"trigger"`
	Run     Run     `toml:"run"`
}

// Load reads every job under cronDir, sorted by name. A missing dir yields no
// jobs rather than an error.
func Load(cronDir string) ([]Job, error) {
	entries, err := os.ReadDir(cronDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cron dir: %w", err)
	}
	var jobs []Job
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		job, err := LoadJob(cronDir, e.Name())
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })
	return jobs, nil
}

// LoadResilient reads every job under cronDir, sorted by name, skipping (rather
// than failing on) any job dir that does not parse or validate. It returns the
// good jobs plus the per-job errors so the scheduler can log them without one
// malformed job stalling the whole tick. A missing dir yields no jobs.
func LoadResilient(cronDir string) ([]Job, []error) {
	entries, err := os.ReadDir(cronDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("read cron dir: %w", err)}
	}
	var (
		jobs []Job
		errs []error
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		job, err := LoadJob(cronDir, e.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })
	return jobs, errs
}

// LoadJob reads and validates a single job by folder name.
func LoadJob(cronDir, name string) (Job, error) {
	dir := filepath.Join(cronDir, name)
	b, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		return Job{}, fmt.Errorf("read job %s: %w", name, err)
	}
	var job Job
	if err := toml.Unmarshal(b, &job); err != nil {
		return Job{}, fmt.Errorf("parse job %s: %w", name, err)
	}
	job.Name = name
	job.Dir = dir
	if err := job.validate(); err != nil {
		return Job{}, fmt.Errorf("job %s: %w", name, err)
	}
	return job, nil
}

func (j Job) validate() error {
	switch j.Run.Kind {
	case "command":
		if j.Run.Command == "" {
			return fmt.Errorf("run.command is required for kind=command")
		}
	case "exec":
		if j.Run.Exec == "" {
			return fmt.Errorf("run.exec is required for kind=exec")
		}
	case "agent":
		if j.Run.Prompt == "" {
			return fmt.Errorf("run.prompt is required for kind=agent")
		}
		switch j.Run.Runner {
		case "", "codex", "claude":
		default:
			return fmt.Errorf("run.runner must be codex or claude, got %q", j.Run.Runner)
		}
	default:
		return fmt.Errorf("run.kind must be agent, command, or exec, got %q", j.Run.Kind)
	}
	return nil
}

// TriggerDesc returns a human-readable description of the job's trigger.
func (j Job) TriggerDesc() string {
	switch {
	case j.Trigger.Schedule != "":
		return "schedule " + j.Trigger.Schedule
	case j.Trigger.On != "":
		return "on " + j.Trigger.On
	default:
		return "manual"
	}
}

// RunDesc returns a human-readable description of what the job runs.
func (j Job) RunDesc() string {
	switch j.Run.Kind {
	case "command":
		return "command: stardust " + j.Run.Command
	case "exec":
		return "exec: " + j.Run.Exec
	case "agent":
		return "agent: " + j.Run.Prompt
	default:
		return j.Run.Kind
	}
}
