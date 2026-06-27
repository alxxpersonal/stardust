package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/clierr"
	"github.com/alxxpersonal/stardust/internal/config"
)

// TestCommandBearingErrorsCarrySuggestion enumerates every command-bearing error
// site and asserts each returns a *clierr.Hint with a non-empty Suggestion. The
// suggestion is the runnable next step the fang handler renders on a Run: line.
func TestCommandBearingErrorsCarrySuggestion(t *testing.T) {
	cases := []struct {
		name    string
		run     func(t *testing.T) error
		wantSug string
	}{
		{
			name: "no vault at the cli boundary",
			run: func(t *testing.T) error {
				t.Chdir(t.TempDir())
				_, err := resolveVault()
				return err
			},
			wantSug: "stardust init",
		},
		{
			name: "check --strict with errors",
			run: func(t *testing.T) error {
				root := t.TempDir()
				_, err := scaffoldVault(t.Context(), root, "off", false)
				require.NoError(t, err)
				t.Setenv("STARDUST_VAULT", root)
				require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/probe\n"), 0o644))
				require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "docs", "specs", "bad-name.md"),
					[]byte("---\ntitle: Bad\ntype: spec\nstatus: Weird\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Bad\n"),
					0o644,
				))
				cmd := newRootCmd()
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				cmd.SetErr(&buf)
				cmd.SetArgs([]string{"check", "--strict", "--output", "plain"})
				return cmd.Execute()
			},
			wantSug: "stardust check --fix",
		},
		{
			name: "registry stale with stale docs",
			run: func(t *testing.T) error {
				root := staleDocsRepo(t)
				t.Setenv("STARDUST_VAULT", root)
				cmd := newRootCmd()
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				cmd.SetErr(&buf)
				cmd.SetArgs([]string{"registry", "stale", "--output", "plain", "--exit-code"})
				return cmd.Execute()
			},
			wantSug: "stardust registry",
		},
		{
			name: "hooks on a non-git dir",
			run: func(t *testing.T) error {
				root := t.TempDir()
				t.Chdir(root)
				initCmd := newInitCmd()
				require.NoError(t, initCmd.Execute())
				cmd := newHooksCmd()
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				cmd.SetErr(&buf)
				cmd.SetArgs([]string{"install"})
				return cmd.Execute()
			},
			wantSug: "git init",
		},
		{
			name: "sync check failed",
			run: func(t *testing.T) error {
				root := syncTestVault(t)
				t.Setenv("STARDUST_VAULT", root)
				cmd := newRootCmd()
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				cmd.SetErr(&buf)
				cmd.SetArgs([]string{"sync", "--check", "--scope", "repo", "--tool", "claude", "--output", "plain"})
				return cmd.Execute()
			},
			wantSug: "stardust sync",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(t)
			require.Error(t, err)
			var h *clierr.Hint
			require.True(t, errors.As(err, &h), "error %v is not a *clierr.Hint", err)
			require.NotEmpty(t, h.Suggestion, "hint carries no suggestion")
			require.Equal(t, tc.wantSug, h.Suggestion)
		})
	}
}

// TestNoVaultWrapPreservesSentinel asserts the cli boundary wrap of ErrNoVault
// keeps the config.ErrNoVault sentinel reachable via errors.Is.
func TestNoVaultWrapPreservesSentinel(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := resolveVault()
	require.Error(t, err)
	require.ErrorIs(t, err, config.ErrNoVault)
}
