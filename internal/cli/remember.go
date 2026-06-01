package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newRememberCmd stores a fact in the vault (add-only, deduped).
func newRememberCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remember <fact>",
		Short: "Store a fact in the vault (add-only, deduped into the nearest note)",
		Long:  "Embeds the fact and appends it to the most similar existing note, or creates\na dated note under memory/. The index updates automatically.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.Remember(ctx, strings.Join(args, " "))
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s %s\n", res.Action, res.Path)
			return nil
		},
	}
}
