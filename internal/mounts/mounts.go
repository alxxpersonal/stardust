// Package mounts is the context-mesh: it federates external sources, each an MCP
// server declared under .stardust/mounts/<name>/config.toml, behind one search.
// Stardust connects as an MCP client, calls each mount's search tool, and the
// caller fuses the results with the local index via RRF. Stardust does not write
// connectors; it aggregates the MCP ecosystem's existing ones.
package mounts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pelletier/go-toml/v2"
)

// Config declares how to launch and query a downstream MCP server. Description
// and Keywords are optional self-description used by query-aware routing to
// decide whether a query is about this mount; a mount with neither is unroutable
// and is therefore always searched (see ADR 0042).
type Config struct {
	Command     string            `toml:"command"` // executable to launch (stdio MCP server)
	Args        []string          `toml:"args"`
	Env         map[string]string `toml:"env"`
	Tool        string            `toml:"tool"`        // the search tool name (default "query")
	Description string            `toml:"description"` // optional: what this mount holds, for routing
	Keywords    []string          `toml:"keywords"`    // optional: routing keywords for lexical matching
}

// Mount is a loaded mount.
type Mount struct {
	Name string
	Cfg  Config
}

// Hit is one result from a mount.
type Hit struct {
	Source  string  `json:"source"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Ref     string  `json:"ref"`
	Score   float64 `json:"score"`
}

// Load reads every mount folder under mountsDir. A missing dir yields no mounts.
func Load(mountsDir string) ([]Mount, error) {
	entries, err := os.ReadDir(mountsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read mounts dir: %w", err)
	}
	var out []Mount
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(mountsDir, e.Name(), "config.toml"))
		if err != nil {
			return nil, fmt.Errorf("read mount %s: %w", e.Name(), err)
		}
		var cfg Config
		if err := toml.Unmarshal(b, &cfg); err != nil {
			return nil, fmt.Errorf("parse mount %s: %w", e.Name(), err)
		}
		if cfg.Command == "" {
			return nil, fmt.Errorf("mount %s: command is required", e.Name())
		}
		if cfg.Tool == "" {
			cfg.Tool = "query"
		}
		out = append(out, Mount{Name: e.Name(), Cfg: cfg})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Search connects to the mount's MCP server, calls its search tool with
// {query, limit}, and returns parsed hits. The connection is per-call.
func (m Mount) Search(ctx context.Context, query string, limit int) ([]Hit, error) {
	cmd := exec.CommandContext(ctx, m.Cfg.Command, m.Cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range m.Cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "stardust", Version: "0.2.0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect mount %s: %w", m.Name, err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      m.Cfg.Tool,
		Arguments: map[string]any{"query": query, "limit": limit},
	})
	if err != nil {
		return nil, fmt.Errorf("call mount %s tool %q: %w", m.Name, m.Cfg.Tool, err)
	}
	return m.Parse(res), nil
}

// Parse extracts hits from a tool result by reading its JSON text content and
// looking for a hits/results array; failing that, the whole text becomes one hit.
func (m Mount) Parse(res *sdkmcp.CallToolResult) []Hit {
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	s := text.String()
	if s == "" {
		return nil
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &doc); err == nil {
		for _, key := range []string{"hits", "results"} {
			if raw, ok := doc[key]; ok {
				var items []map[string]any
				if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
					return m.fromItems(items)
				}
			}
		}
	}
	return []Hit{{Source: m.Name, Snippet: truncate(s, 400)}}
}

func (m Mount) fromItems(items []map[string]any) []Hit {
	hits := make([]Hit, 0, len(items))
	for _, it := range items {
		hits = append(hits, Hit{
			Source:  m.Name,
			Title:   str(it, "title"),
			Snippet: firstNonEmpty(str(it, "snippet"), str(it, "body"), str(it, "text")),
			Ref:     firstNonEmpty(str(it, "path"), str(it, "ref"), str(it, "id"), str(it, "url")),
			Score:   num(it, "score"),
		})
	}
	return hits
}

// --- Helpers ---

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func num(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}
