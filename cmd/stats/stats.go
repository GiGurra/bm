package stats

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/gigurra/bm/pkg/config"
	"github.com/gigurra/bm/pkg/db"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

type Params struct {
	Profile string `short:"p" optional:"true" env:"BM_PROFILE" help:"Filter by profile (name, email, or source ID; 'all' for all profiles)"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "stats",
		Short: "Show bookmark database statistics",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(profileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			// Resolve profiles from CLI/env/config
			resolved, err := config.ResolveProfiles(params.Profile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Resolve source IDs for DB queries
			var dbSources []string
			if resolved != nil {
				for _, p := range resolved {
					dbSources = append(dbSources, p.SourceID())
				}
			}

			// Per-profile: Chrome JSON vs imported vs fetched vs indexed
			profiles := resolved
			if profiles == nil {
				profiles, _ = chrome.DiscoverProfiles()
			}
			dbProfileStats, _ := db.ListProfileStats()

			dbBySource := make(map[string]db.ProfileStats)
			for _, s := range dbProfileStats {
				dbBySource[s.Source] = s
			}

			// Collect deduped Chrome bookmarks for per-year counts.
			chromeByYear := make(map[string]int)

			if len(profiles) > 0 || (resolved == nil && len(dbProfileStats) > 0) {
				fmt.Println("Profiles:")
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Profile", "In Chrome", "Imported", "Fetched", "Indexed"})

				var totalChrome, totalImported, totalFetched, totalIndexed int
				for _, p := range profiles {
					chromeCount := 0
					if bookmarks, err := chrome.ParseBookmarksFile(p.Path); err == nil {
						bookmarks = chrome.Dedup(bookmarks)
						chromeCount = len(bookmarks)
						for _, b := range bookmarks {
							year := "?"
							if len(b.DateAdded) >= 4 {
								year = b.DateAdded[:4]
							}
							chromeByYear[year]++
						}
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
				if resolved == nil {
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
				}

				t.AppendFooter(table.Row{"Total", totalChrome, totalImported, totalFetched, totalIndexed})
				t.Render()
			}

			// By year
			yearStats, err := db.ListYearStats(dbSources)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(yearStats) > 0 || len(chromeByYear) > 0 {
				// Merge DB year stats with Chrome year counts
				type yearRow struct {
					year                                      string
					chrome, imported, fetched, errors, indexed int
				}
				rowByYear := make(map[string]*yearRow)
				for _, s := range yearStats {
					rowByYear[s.Year] = &yearRow{year: s.Year, imported: s.Total, fetched: s.Fetched, errors: s.Errors, indexed: s.Indexed}
				}
				for year, count := range chromeByYear {
					if r, ok := rowByYear[year]; ok {
						r.chrome = count
					} else {
						rowByYear[year] = &yearRow{year: year, chrome: count}
					}
				}

				// Sort by year
				var years []string
				for y := range rowByYear {
					years = append(years, y)
				}
				sort.Strings(years)

				fmt.Println("\nBy year:")
				t := table.NewWriter()
				t.SetOutputMirror(os.Stdout)
				t.SetStyle(table.StyleLight)
				t.AppendHeader(table.Row{"Year", "In Chrome", "Imported", "Fetched", "Errors", "Indexed"})

				var grandChrome, grandTotal, grandFetched, grandErrors, grandIndexed int
				for _, y := range years {
					r := rowByYear[y]
					t.AppendRow(table.Row{r.year, r.chrome, r.imported, r.fetched, r.errors, r.indexed})
					grandChrome += r.chrome
					grandTotal += r.imported
					grandFetched += r.fetched
					grandErrors += r.errors
					grandIndexed += r.indexed
				}
				t.AppendFooter(table.Row{"Total", grandChrome, grandTotal, grandFetched, grandErrors, grandIndexed})
				t.Render()
			}

			// Fetch status breakdown
			fetchStats, err := db.ListFetchStatusStats(dbSources)
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

func profileAlternatives(_ *cobra.Command, _ []string, toComplete string) []string {
	profiles, err := chrome.DiscoverProfiles()
	if err != nil {
		return nil
	}
	alts := []string{"all"}
	for _, p := range profiles {
		for _, candidate := range []string{p.UserName, p.SourceID(), p.DirName} {
			if candidate != "" && strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(toComplete)) {
				alts = append(alts, candidate)
			}
		}
	}
	return alts
}
