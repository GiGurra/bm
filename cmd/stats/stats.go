package stats

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/db"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

type Params struct{}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "stats",
		Short: "Show bookmark database statistics",
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			// Overview
			total, _ := db.CountBookmarks()
			fetched, _ := db.CountFetched()

			fmt.Printf("Bookmarks: %d total, %d fetched\n\n", total, fetched)

			// By year
			yearStats, err := db.ListYearStats()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(yearStats) > 0 {
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Year", "Total", "Fetched", "Errors", "Indexed"})

				var grandTotal, grandFetched, grandErrors, grandIndexed int
				for _, s := range yearStats {
					t.AppendRow(table.Row{s.Year, s.Total, s.Fetched, s.Errors, s.Indexed})
					grandTotal += s.Total
					grandFetched += s.Fetched
					grandErrors += s.Errors
					grandIndexed += s.Indexed
				}
				t.AppendFooter(table.Row{"Total", grandTotal, grandFetched, grandErrors, grandIndexed})
				t.Render()
			}

			// Fetch status breakdown
			fetchStats, err := db.ListFetchStatusStats()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(fetchStats) > 0 {
				fmt.Println()
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Fetch Status", "Count"})
				for _, s := range fetchStats {
					t.AppendRow(table.Row{s.Status, s.Count})
				}
				t.Render()
			}

			// Per-profile
			profileStats, err := db.ListProfileStats()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(profileStats) > 0 {
				fmt.Println()
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Profile", "Total", "Fetched", "Errors", "Indexed"})
				for _, s := range profileStats {
					name := s.SourceName
					if name == "" {
						name = s.Source
					}
					t.AppendRow(table.Row{name, s.Total, s.Fetched, s.Errors, s.Indexed})
				}
				t.Render()
			}
		},
	}.ToCobra()
}
