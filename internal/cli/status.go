package cli

import (
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/ui"
)

// newStatusCmd reports vault initialization, kind, collections, and index health.
func newStatusCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report vault initialization, kind, collections, and index health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(cmd, output)
		},
	}
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, json")
	return cmd
}

// runStatus resolves the start directory (STARDUST_VAULT or cwd, same precedence
// as openService), gathers the full state probe, and renders it: indented
// ANSI-free JSON for --output json, otherwise a compact human-readable block.
func runStatus(cmd *cobra.Command, output string) error {
	start := os.Getenv("STARDUST_VAULT")
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working dir: %w", err)
		}
		start = cwd
	}

	st, err := service.GatherStatus(cmd.Context(), start)
	if err != nil {
		return err
	}

	if output == "json" {
		return emitJSON(cmd.OutOrStdout(), st)
	}
	writeStatusHuman(cmd.OutOrStdout(), st)
	return nil
}

// writeStatusHuman renders the human-readable status block to w styled with the
// cosmic lipgloss palette, routed through a colorprofile writer so the styling
// shows on a TTY and degrades to plain text when piped. The --output json path
// is separate and stays raw.
func writeStatusHuman(w io.Writer, st service.VaultStatus) {
	out := colorprofile.NewWriter(w, os.Environ())
	var (
		head  = lipgloss.NewStyle().Foreground(ui.Primary).Bold(true)
		label = lipgloss.NewStyle().Foreground(ui.Muted)
		value = lipgloss.NewStyle().Foreground(ui.Text)
		hi    = lipgloss.NewStyle().Foreground(ui.Accent)
		name  = lipgloss.NewStyle().Foreground(ui.Secondary)
	)

	fmt.Fprintln(out, head.Render("stardust status"))
	fmt.Fprintf(out, "  %s %s\n", label.Render("root:       "), value.Render(st.Root))
	fmt.Fprintf(out, "  %s %s\n", label.Render("initialized:"), hi.Render(yesNo(st.Initialized)))
	fmt.Fprintf(out, "  %s %s\n", label.Render("kind:       "), hi.Render(st.Kind))

	if !st.Initialized {
		fmt.Fprintf(out, "  %s %s\n", label.Render("hint:       "), value.Render(st.Hint))
		return
	}

	if st.Source.Path != "" {
		fmt.Fprintf(out, "  %s %s\n", label.Render("source root:"), value.Render(fmt.Sprintf("%s (%s)", st.Source.Path, st.Source.Origin)))
	}

	if len(st.Collections) > 0 {
		fmt.Fprintln(out, label.Render("  collections:"))
		width := 0
		for _, c := range st.Collections {
			if len(c.Name) > width {
				width = len(c.Name)
			}
		}
		for _, c := range st.Collections {
			fmt.Fprintf(out, "    %s  %s\n", name.Render(fmt.Sprintf("%-*s", width, c.Name)), value.Render(fmt.Sprintf("%d", c.Records)))
		}
	}

	fmt.Fprintln(out, label.Render("  index:"))
	fmt.Fprintf(out, "    %s %s\n", label.Render("notes:    "), value.Render(fmt.Sprintf("%d", st.Index.Notes)))
	fmt.Fprintf(out, "    %s %s\n", label.Render("vectors:  "), value.Render(vectorsLine(st.Index)))
	fmt.Fprintf(out, "    %s %s\n", label.Render("freshness:"), value.Render(freshnessLine(st.Index)))
}

// yesNo renders a bool as the status block's yes or no.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// vectorsLine renders the vectors field as on, or off with its reason.
func vectorsLine(h service.IndexHealth) string {
	if h.Vectors {
		return "on"
	}
	if h.VectorsReason != "" {
		return fmt.Sprintf("off (%s)", h.VectorsReason)
	}
	return "off"
}

// freshnessLine renders the index freshness as commits-behind-HEAD when git is
// available, otherwise an explicit unknown.
func freshnessLine(h service.IndexHealth) string {
	if h.HasCommitsBehind {
		return fmt.Sprintf("%d commits behind HEAD", h.CommitsBehind)
	}
	return "unknown (no git or unindexed)"
}
