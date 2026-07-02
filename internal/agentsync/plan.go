package agentsync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options controls sync planning and execution.
type Options struct {
	ConfigPath string
	Scope      Scope
	Tools      []Tool
	DryRun     bool
	Check      bool
	Repair     bool
}

// Action describes one planned sync operation.
type Action struct {
	Kind     Kind   `json:"kind"`
	ItemName string `json:"item_name"`
	Tool     Tool   `json:"tool"`
	Scope    Scope  `json:"scope"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Mode     string `json:"mode"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}

// Plan is a complete set of sync actions plus drift counts.
type Plan struct {
	Actions   []Action `json:"actions"`
	Missing   int      `json:"missing"`
	Drift     int      `json:"drift"`
	Conflicts int      `json:"conflicts"`
	Repair    bool     `json:"repair"`
}

// BuildPlan inspects target paths and returns the actions required to sync items.
func BuildPlan(cfg Config, items []Item, opts Options) (Plan, error) {
	plan := Plan{Repair: opts.Repair}
	for _, target := range cfg.Targets {
		if !scopeAllowed(target.Scope, opts.Scope) || !toolAllowed(target.Tool, opts.Tools) {
			continue
		}
		for _, item := range items {
			if item.Source.ImportOnly {
				continue
			}
			if !itemTargetsTool(item, target.Tool) {
				continue
			}
			if item.Kind == KindRules && target.RulesPath == "" {
				continue
			}
			action, err := buildAction(target, item)
			if err != nil {
				return Plan{}, err
			}
			switch action.Status {
			case "create":
				plan.Missing++
			case "drift":
				plan.Drift++
			case "conflict":
				plan.Conflicts++
			}
			plan.Actions = append(plan.Actions, action)
		}
	}
	sort.SliceStable(plan.Actions, func(i, j int) bool {
		a, b := plan.Actions[i], plan.Actions[j]
		if a.Tool != b.Tool {
			return a.Tool < b.Tool
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.ItemName < b.ItemName
	})
	return plan, nil
}

// Markdown renders the sync plan as a compact markdown table.
func (p Plan) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Agent Sync Plan\n\n")
	fmt.Fprintf(&b, "missing: %d\n", p.Missing)
	fmt.Fprintf(&b, "drift: %d\n", p.Drift)
	fmt.Fprintf(&b, "conflicts: %d\n\n", p.Conflicts)
	fmt.Fprintf(&b, "| kind | item | tool | scope | status | target | reason |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|---|---|\n")
	for _, action := range p.Actions {
		fmt.Fprintf(
			&b,
			"| %s | %s | %s | %s | %s | `%s` | %s |\n",
			action.Kind,
			action.ItemName,
			action.Tool,
			action.Scope,
			action.Status,
			action.Target,
			action.Reason,
		)
	}
	return b.String()
}

func buildAction(target Target, item Item) (Action, error) {
	if item.Kind == KindRules {
		return buildRulesAction(target, item)
	}
	action := Action{
		Kind:     item.Kind,
		ItemName: item.Name,
		Tool:     target.Tool,
		Scope:    target.Scope,
		Source:   filepath.Clean(item.SourcePath),
		Target:   itemTargetPath(target, item),
		Mode:     target.Mode,
	}
	if action.Mode == "" {
		action.Mode = "symlink"
	}

	info, err := os.Lstat(action.Target)
	if err != nil {
		if os.IsNotExist(err) {
			action.Status = "create"
			action.Reason = "missing"
			return action, nil
		}
		return Action{}, fmt.Errorf("stat sync target %s: %w", action.Target, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		action.Status = "conflict"
		action.Reason = "target exists and is not a symlink"
		return action, nil
	}

	link, err := os.Readlink(action.Target)
	if err != nil {
		return Action{}, fmt.Errorf("read sync target %s: %w", action.Target, err)
	}
	if sameTarget(action.Target, link, action.Source) {
		action.Status = "ok"
		action.Reason = "linked"
		return action, nil
	}
	action.Status = "drift"
	action.Reason = "points to " + filepath.Clean(link)
	return action, nil
}

// buildRulesAction classifies a rules item against a compose target. A missing
// file or a file without our block is create; a present block equal to the
// rendered body is ok; a present block that differs is drift. Rules never
// produce a conflict: composing into a user-owned file is always safe (ADR 0007).
func buildRulesAction(target Target, item Item) (Action, error) {
	action := Action{
		Kind:     item.Kind,
		ItemName: item.Name,
		Tool:     target.Tool,
		Scope:    target.Scope,
		Source:   filepath.Clean(item.SourcePath),
		Target:   target.RulesPath,
		Mode:     "compose",
	}

	raw, err := os.ReadFile(item.SourcePath)
	if err != nil {
		return Action{}, fmt.Errorf("read rules source %s: %w", item.SourcePath, err)
	}
	rendered, err := renderRules(target.Tool, string(raw))
	if err != nil {
		return Action{}, err
	}

	existing, err := os.ReadFile(target.RulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			action.Status = "create"
			action.Reason = "missing"
			return action, nil
		}
		return Action{}, fmt.Errorf("read rules target %s: %w", target.RulesPath, err)
	}

	current, found := extractRulesBlock(string(existing))
	if !found {
		action.Status = "create"
		action.Reason = "block absent"
		return action, nil
	}
	if strings.TrimSpace(current) == strings.TrimSpace(rendered) {
		action.Status = "ok"
		action.Reason = "composed"
		return action, nil
	}
	action.Status = "drift"
	action.Reason = "block differs from source"
	return action, nil
}

func itemTargetPath(target Target, item Item) string {
	switch item.Kind {
	case KindAgent:
		return filepath.Join(target.AgentsPath, item.Name+".md")
	case KindRules:
		return target.RulesPath
	default:
		return filepath.Join(target.SkillsPath, item.Name)
	}
}

func scopeAllowed(target Scope, requested Scope) bool {
	return requested == "" || requested == ScopeAll || requested == target
}

func toolAllowed(target Tool, requested []Tool) bool {
	if len(requested) == 0 {
		return true
	}
	for _, tool := range requested {
		if tool == target {
			return true
		}
	}
	return false
}

func itemTargetsTool(item Item, tool Tool) bool {
	for _, target := range item.Targets {
		if target == tool {
			return true
		}
	}
	return false
}

func sameTarget(linkPath, linkValue, source string) bool {
	linkTarget := filepath.Clean(linkValue)
	if !filepath.IsAbs(linkTarget) {
		linkTarget = filepath.Clean(filepath.Join(filepath.Dir(linkPath), linkTarget))
	}
	source = filepath.Clean(source)
	if absLink, err := filepath.Abs(linkTarget); err == nil {
		linkTarget = absLink
	}
	if absSource, err := filepath.Abs(source); err == nil {
		source = absSource
	}
	return linkTarget == source
}
