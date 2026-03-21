package importcmd

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/gigurra/bm/pkg/db"
	"github.com/spf13/cobra"
)

type Params struct {
	Path        string `pos:"true" optional:"true" help:"Path to Chrome Bookmarks JSON file (auto-detected if omitted)"`
	AllProfiles bool   `short:"a" long:"all-profiles" help:"Import from all Chrome profiles"`
	Profile     string `short:"p" optional:"true" help:"Chrome profile name (e.g. 'Default', 'Profile 1')"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "import",
		Short: "Import bookmarks from Chrome",
		Long:  "Reads Chrome's Bookmarks JSON file and imports URLs into the database.",
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			if params.Path != "" {
				importFile(params.Path)
				return
			}

			if params.Profile != "" {
				profiles, err := chrome.DiscoverProfiles()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				for _, p := range profiles {
					if p.DirName == params.Profile || p.UserName == params.Profile || p.Name == params.Profile {
						importProfile(p)
						return
					}
				}
				fmt.Fprintf(os.Stderr, "Profile %q not found. Available profiles:\n", params.Profile)
				for _, p := range profiles {
					fmt.Fprintf(os.Stderr, "  - %s\n", p.DisplayName())
				}
				os.Exit(1)
				return
			}

			// Default (also --all-profiles): import all profiles
			importAllProfiles()
		},
	}.ToCobra()
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

	sourceID := p.SourceID()
	sourceName := p.DisplayName()

	imported := 0
	for _, b := range bookmarks {
		err := db.UpsertBookmark(&db.Bookmark{
			URL:           b.URL,
			Title:         b.Title,
			FolderPath:    b.FolderPath,
			Source:        sourceID,
			SourceName:    sourceName,
			ChromeAddedAt: b.DateAdded,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing %s: %v\n", b.URL, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d bookmarks from %s\n", imported, sourceName)
	return imported
}

func importFile(path string) int {
	bookmarks, err := chrome.ParseBookmarksFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		return 0
	}

	imported := 0
	for _, b := range bookmarks {
		err := db.UpsertBookmark(&db.Bookmark{
			URL:           b.URL,
			Title:         b.Title,
			FolderPath:    b.FolderPath,
			Source:        "chrome:file",
			ChromeAddedAt: b.DateAdded,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing %s: %v\n", b.URL, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d bookmarks from %s\n", imported, path)
	return imported
}
