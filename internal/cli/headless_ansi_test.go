package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPipedJSONOutputHasZeroANSI proves the markdown-safe boundary: a data
// command with --output json, written to a real os.Pipe (a non-tty file
// descriptor, not a bytes.Buffer), carries zero ANSI escape bytes and parses as
// JSON. fang styles only help and errors; the data writer on cmd.OutOrStdout is
// never routed through fang, so the agent contract holds structurally.
func TestPipedJSONOutputHasZeroANSI(t *testing.T) {
	root := governsDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)

	pr, pw, err := os.Pipe()
	require.NoError(t, err)

	cmd := newRootCmd()
	cmd.SetOut(pw)
	cmd.SetErr(pw)
	cmd.SetArgs([]string{"registry", "governs", "internal/service/check.go", "--output", "json"})

	// read concurrently so a full pipe buffer never blocks the writer.
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

	var v any
	require.NoError(t, json.Unmarshal(out, &v), "piped output must parse as JSON, got %q", out)
}
