package list

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/gigurra/bm/pkg/config"
	"github.com/gigurra/bm/pkg/db"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type Params struct {
	Folder  string `short:"f" optional:"true" help:"Filter by folder path (substring match)"`
	Limit   int    `short:"n" help:"Max results" default:"50"`
	Watch   bool   `short:"w" long:"watch" help:"Interactive mode with search"`
	Profile string `short:"p" optional:"true" env:"BM_PROFILE" help:"Filter by profile (name, email, or source ID; 'all' for all profiles)"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "list",
		Short: "List bookmarks",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(profileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			sources, err := config.ResolveSourceIDs(params.Profile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if params.Watch {
				if err := RunWatch(sources); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				return
			}

			var bookmarks []db.Bookmark
			bookmarks, err = db.ListBookmarksBySources(sources)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Filter by folder if specified
			if params.Folder != "" {
				var filtered []db.Bookmark
				for _, b := range bookmarks {
					if containsIgnoreCase(b.FolderPath, params.Folder) {
						filtered = append(filtered, b)
					}
				}
				bookmarks = filtered
			}

			// Sort by date desc (newest first)
			sort.Slice(bookmarks, func(i, j int) bool {
				return bookmarks[i].ChromeAddedAt > bookmarks[j].ChromeAddedAt
			})

			total := len(bookmarks)

			if params.Limit > 0 && params.Limit < len(bookmarks) {
				bookmarks = bookmarks[:params.Limit]
			}

			if len(bookmarks) == 0 {
				fmt.Println("No bookmarks found.")
				return
			}

			tw := getTerminalWidth()
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleLight)
			t.SetAllowedRowLength(tw)

			// Allocate widths proportionally: title ~28%, URL ~35%, folder ~22%, date fixed
			usable := tw - 22 // margins + status col + date col + separators
			titleW := max(usable*28/100, 15)
			urlW := max(usable*40/100, 20)
			folderW := max(usable*25/100, 10)

			t.SetColumnConfigs([]table.ColumnConfig{
				{Number: 2, WidthMax: titleW, WidthMaxEnforcer: truncateWithEllipsis},
				{Number: 3, WidthMax: urlW, WidthMaxEnforcer: truncateWithEllipsis},
				{Number: 4, WidthMax: folderW, WidthMaxEnforcer: truncateWithEllipsis},
			})
			t.AppendHeader(table.Row{"", "Title", "URL", "Folder", "Added"})

			for _, b := range bookmarks {
				status := " "
				if b.FetchStatus == "ok" {
					status = "+"
				} else if b.FetchStatus != "" {
					status = "!"
				}
				t.AppendRow(table.Row{status, b.Title, b.URL, b.FolderPath, formatDateShort(b.ChromeAddedAt)})
			}

			t.AppendFooter(table.Row{"", fmt.Sprintf("Showing %d of %d", len(bookmarks), total), "", "", ""})
			t.Render()
		},
	}.ToCobra()
}

func getTerminalWidth() int {
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	if width, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && width > 0 {
		return width
	}
	return 120
}

func formatDateShort(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func truncateWithEllipsis(col string, maxLen int) string {
	if len(col) <= maxLen {
		return col
	}
	if maxLen <= 3 {
		return col[:maxLen]
	}
	return col[:maxLen-3] + "..."
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

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findIgnoreCase(s, substr))
}

func findIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			a, b := s[i+j], substr[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
