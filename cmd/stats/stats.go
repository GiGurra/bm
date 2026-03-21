package stats

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
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
			// Per-profile: Chrome JSON vs imported vs fetched vs indexed
			profiles, _ := chrome.DiscoverProfiles()
			dbProfileStats, _ := db.ListProfileStats()

			dbBySource := make(map[string]db.ProfileStats)
			for _, s := range dbProfileStats {
				dbBySource[s.Source] = s
			}

			if len(profiles) > 0 || len(dbProfileStats) > 0 {
				fmt.Println("Profiles:")
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Profile", "In Chrome", "Imported", "Fetched", "Indexed"})

				var totalChrome, totalImported, totalFetched, totalIndexed int
				for _, p := range profiles {
					chromeCount := 0
					if bookmarks, err := chrome.ParseBookmarksFile(p.Path); err == nil {
						chromeCount = len(bookmarks)
					}
					sourceID := p.SourceID()
					dbs := dbBySource[sourceID]
					delete(dbBySource, sourceID)

					name := p.DisplayName()
					t.AppendRow(table.Row{name, chromeCount, dbs.Total, dbs.Fetched, dbs.Indexed})
					totalChrome += chromeCount
					totalImported += dbs.Total
					totalFetched += dbs.Fetched
					totalIndexed += dbs.Indexed
				}

				// DB profiles not matched to a Chrome profile (e.g. removed profiles)
				for _, dbs := range dbBySource {
					name := dbs.SourceName
					if name == "" {
						name = dbs.Source
					}
					t.AppendRow(table.Row{name, "-", dbs.Total, dbs.Fetched, dbs.Indexed})
					totalImported += dbs.Total
					totalFetched += dbs.Fetched
					totalIndexed += dbs.Indexed
				}

				t.AppendFooter(table.Row{"Total", totalChrome, totalImported, totalFetched, totalIndexed})
				t.Render()
			}

			// By year
			yearStats, err := db.ListYearStats()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(yearStats) > 0 {
				fmt.Println("\nBy year:")
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Year", "Imported", "Fetched", "Errors", "Indexed"})

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
				fmt.Println("\nFetch status:")
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Status", "Count"})
				for _, s := range fetchStats {
					t.AppendRow(table.Row{s.Status, s.Count})
				}
				t.Render()
			}
		},
	}.ToCobra()
}
