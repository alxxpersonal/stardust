package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/cron"
)

// newCronCmd groups the declarative cron-job commands.
func newCronCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "List and run declarative cron jobs",
	}
	cmd.AddCommand(newCronListCmd(), newCronRunCmd())
	return cmd
}

func newCronListCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured cron jobs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			vc, err := resolveVault()
			if err != nil {
				return err
			}
			jobs, err := cron.Load(vc.Layout.CronJobs())
			if err != nil {
				return err
			}
			if output == "json" {
				return emitJSON(cmd.OutOrStdout(), jobs)
			}
			var b strings.Builder
			fmt.Fprintf(&b, "# Cron jobs (%d)\n\n", len(jobs))
			if len(jobs) == 0 {
				b.WriteString("_None. Add `.stardust/cron-jobs/<name>/config.toml` to define one._\n")
			}
			for _, j := range jobs {
				fmt.Fprintf(&b, "- **%s** - %s, %s\n", j.Name, j.TriggerDesc(), j.RunDesc())
			}
			emitMarkdown(cmd.OutOrStdout(), b.String(), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}

func newCronRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <job>",
		Short: "Run a cron job now",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vc, err := resolveVault()
			if err != nil {
				return err
			}
			job, err := cron.LoadJob(vc.Layout.CronJobs(), args[0])
			if err != nil {
				return err
			}
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate executable: %w", err)
			}
			return job.Execute(cmd.Context(), exe, vc.Layout.Root, os.Stderr)
		},
	}
}
