package profile

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/gigurra/bm/pkg/db"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage browser profiles",
	}
	cmd.AddCommand(listCmd())
	return cmd
}

type ListParams struct{}

func listCmd() *cobra.Command {
	return boa.CmdT[ListParams]{
		Use:   "list",
		Short: "List profiles with stats",
		RunFunc: func(params *ListParams, cmd *cobra.Command, args []string) {
			// Get stats from DB
			dbStats, err := db.ListProfileStats()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			dbBySource := make(map[string]db.ProfileStats)
			for _, s := range dbStats {
				dbBySource[s.Source] = s
			}

			type profileRow struct {
				displayName string
				sourceID    string
				stats       db.ProfileStats
			}
			var rows []profileRow
			seen := make(map[string]bool)

			if profiles, err := chrome.DiscoverProfiles(); err == nil {
				for _, p := range profiles {
					sid := p.SourceID()
					seen[sid] = true
					s := dbBySource[sid]
					rows = append(rows, profileRow{
						displayName: p.DisplayName(),
						sourceID:    sid,
						stats:       s,
					})
				}
			}

			for _, s := range dbStats {
				if !seen[s.Source] {
					name := s.SourceName
					if name == "" {
						name = s.Source
					}
					rows = append(rows, profileRow{
						displayName: name,
						sourceID:    s.Source,
						stats:       s,
					})
				}
			}

			if len(rows) == 0 {
				fmt.Println("No profiles found. Run 'bm import' first.")
				return
			}

			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleLight)
			t.AppendHeader(table.Row{"Profile", "Source ID", "Total", "Fetched", "Errors", "Indexed"})

			var grandTotal, grandFetched, grandErrors, grandIndexed int
			for _, r := range rows {
				t.AppendRow(table.Row{
					r.displayName, r.sourceID,
					r.stats.Total, r.stats.Fetched, r.stats.Errors, r.stats.Indexed,
				})
				grandTotal += r.stats.Total
				grandFetched += r.stats.Fetched
				grandErrors += r.stats.Errors
				grandIndexed += r.stats.Indexed
			}

			if len(rows) > 1 {
				t.AppendFooter(table.Row{"Total", "", grandTotal, grandFetched, grandErrors, grandIndexed})
			}

			t.Render()
		},
	}.ToCobra()
}
