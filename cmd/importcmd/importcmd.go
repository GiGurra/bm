package importcmd

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/cmd/common"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/gigurra/bm/pkg/config"
	"github.com/gigurra/bm/pkg/db"
	"github.com/spf13/cobra"
)

type Params struct {
	Path        string `pos:"true" optional:"true" help:"Path to Chrome Bookmarks JSON file (auto-detected if omitted)"`
	AllProfiles bool   `short:"a" long:"all-profiles" help:"Import from all Chrome profiles"`
	Profile     string `short:"p" optional:"true" env:"BM_PROFILE" help:"Chrome profile (email, gaia ID, or dir name; 'all' for all profiles)"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "import",
		Short: "Import bookmarks from Chrome",
		Long:  "Reads Chrome's Bookmarks JSON file and imports URLs into the database.",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(common.ProfileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			Run(params.Path, params.Profile)
		},
	}.ToCobra()
}

func Run(path, profile string) {
	if path != "" {
		importFile(path)
		return
	}

	resolved, err := config.ResolveProfiles(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if resolved == nil {
		importAllProfiles()
		return
	}

	totalImported := 0
	for _, p := range resolved {
		count := importProfile(p)
		totalImported += count
	}
	if len(resolved) > 1 {
		fmt.Printf("\nTotal: %d bookmarks from %d profiles\n", totalImported, len(resolved))
	}
}

func importAllProfiles() {
	profiles, err := chrome.DiscoverProfiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering profiles: %v\n", err)
		os.Exit(1)
	}

	if len(profiles) == 0 {
		fmt.Fprintln(os.Stderr, "No Chrome profiles found.")
		os.Exit(1)
	}

	fmt.Printf("Found %d Chrome profile(s):\n", len(profiles))
	for _, p := range profiles {
		fmt.Printf("  - %s [%s]\n", p.DisplayName(), p.SourceID())
	}
	fmt.Println()

	totalImported := 0
	for _, p := range profiles {
		count := importProfile(p)
		totalImported += count
	}

	if len(profiles) > 1 {
		fmt.Printf("\nTotal: %d bookmarks from %d profiles\n", totalImported, len(profiles))
	}
}

func importProfile(p chrome.Profile) int {
	bookmarks, err := chrome.ParseBookmarksFile(p.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", p.Path, err)
		return 0
	}

	bookmarks = chrome.Dedup(bookmarks)
	sourceID := p.SourceID()
	sourceName := p.DisplayName()

	dbBookmarks := make([]*db.Bookmark, len(bookmarks))
	for i, b := range bookmarks {
		dbBookmarks[i] = &db.Bookmark{
			URL:           b.URL,
			Title:         b.Title,
			FolderPath:    b.FolderPath,
			Source:        sourceID,
			SourceName:    sourceName,
			ChromeAddedAt: b.DateAdded,
		}
	}

	inserted, updated, deleted, total, err := db.BulkUpsertBookmarks(dbBookmarks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error importing from %s: %v\n", sourceName, err)
		return 0
	}

	unchanged := total - inserted - updated
	fmt.Printf("Imported from %s: %d new, %d updated, %d deleted, %d unchanged (total %d)\n",
		sourceName, inserted, updated, deleted, unchanged, total)
	return inserted + updated
}


func importFile(path string) int {
	bookmarks, err := chrome.ParseBookmarksFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		return 0
	}

	bookmarks = chrome.Dedup(bookmarks)
	dbBookmarks := make([]*db.Bookmark, len(bookmarks))
	for i, b := range bookmarks {
		dbBookmarks[i] = &db.Bookmark{
			URL:           b.URL,
			Title:         b.Title,
			FolderPath:    b.FolderPath,
			Source:        "chrome:file",
			ChromeAddedAt: b.DateAdded,
		}
	}

	inserted, updated, deleted, total, err := db.BulkUpsertBookmarks(dbBookmarks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error importing from %s: %v\n", path, err)
		return 0
	}

	unchanged := total - inserted - updated
	fmt.Printf("Imported from %s: %d new, %d updated, %d deleted, %d unchanged (total %d)\n",
		path, inserted, updated, deleted, unchanged, total)
	return inserted + updated
}
