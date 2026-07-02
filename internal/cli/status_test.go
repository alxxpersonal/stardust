package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
)

// TestStatusJSONHasZeroANSI mirrors TestPipedJSONOutputHasZeroANSI: status with
// --output json, written to a real os.Pipe, carries zero ANSI escape bytes and
// parses as JSON carrying an "initialized" key.
func TestStatusJSONHasZeroANSI(t *testing.T) {
	root := governsDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)

	pr, pw, err := os.Pipe()
	require.NoError(t, err)

	cmd := newRootCmd()
	cmd.SetOut(pw)
	cmd.SetErr(pw)
	cmd.SetArgs([]string{"status", "--output", "json"})

	done := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(pr)
		done <- data
	}()

	require.NoError(t, cmd.Execute())
	require.NoError(t, pw.Close())
	out := <-done
	require.NoError(t, pr.Close())

	require.False(t, bytes.Contains(out, []byte("\x1b[")), "piped JSON must carry zero ANSI escape bytes, got %q", out)

	var v map[string]any
	require.NoError(t, json.Unmarshal(out, &v), "piped output must parse as JSON, got %q", out)
	_, ok := v["initialized"]
	require.True(t, ok, "status JSON must carry an initialized key, got %q", out)
}

// TestStatusHumanUninitialized asserts the default human block reports an
// uninitialized directory and points at stardust init.
func TestStatusHumanUninitialized(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("STARDUST_VAULT", dir)

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	require.Contains(t, out, "initialized: no")
	require.Contains(t, out, "stardust init")
}

// TestStatusHumanShowsConfiguredSourceRoot asserts an explicit source_root renders
// as a source root line tagged configured.
func TestStatusHumanShowsConfiguredSourceRoot(t *testing.T) {
	root := t.TempDir()
	layout := config.Layout{Root: root}
	require.NoError(t, os.MkdirAll(layout.Cache(), 0o755))
	cfg := config.Default()
	cfg.SourceRoot = filepath.Join(root, "explicit-source")
	require.NoError(t, config.Save(layout.Config(), cfg))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644))
	t.Setenv("STARDUST_VAULT", root)

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	require.Contains(t, out, "source root:")
	require.Contains(t, out, "(configured)")
}

// TestStatusHumanOmitsSourceRootWhenUnbound asserts a plain repo with no bound
// source root does not render the source root line.
func TestStatusHumanOmitsSourceRootWhenUnbound(t *testing.T) {
	root := governsDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())

	require.NotContains(t, buf.String(), "source root:")
}
