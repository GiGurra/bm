package fetch

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/gigurra/bm/pkg/config"
	"github.com/gigurra/bm/pkg/db"
	"github.com/gigurra/bm/pkg/fetcher"
	"github.com/spf13/cobra"
)

type Params struct {
	All     bool   `short:"a" help:"Re-fetch all bookmarks, not just unfetched ones"`
	Limit   int    `short:"n" help:"Max number of bookmarks to fetch" default:"0"`
	Delay   int    `short:"d" help:"Delay in milliseconds between fetches" default:"500"`
	MaxAge  string `long:"max-age" help:"Skip bookmarks older than this (e.g. 1y, 6m, 90d)" default:"1y"`
	Profile string `short:"p" optional:"true" env:"BM_PROFILE" help:"Filter by profile (name, email, or source ID; 'all' for all profiles)"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "fetch",
		Short: "Fetch page content for bookmarks",
		Long:  "Downloads and extracts text content from bookmarked URLs.\nSkips bookmarks older than --max-age and marks HTTP errors (404, 403, etc.) as unfetchable.",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(profileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			Run(params.Profile, params.MaxAge, params.All, params.Limit, params.Delay)
		},
	}.ToCobra()
}

func Run(profile, maxAge string, all bool, limit, delay int) {
	sources, err := config.ResolveSourceIDs(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var bookmarks []db.Bookmark
	if all {
		bookmarks, err = db.ListBookmarksBySources(sources)
	} else {
		bookmarks, err = db.ListFetchableBySources(sources)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Apply age cutoff
	cutoff := parseDuration(maxAge)
	if cutoff > 0 {
		cutoffTime := time.Now().Add(-cutoff)
		var filtered []db.Bookmark
		skipped := 0
		for _, b := range bookmarks {
			age := bookmarkAge(b)
			if !age.IsZero() && age.Before(cutoffTime) {
				skipped++
				continue
			}
			filtered = append(filtered, b)
		}
		if skipped > 0 {
			fmt.Printf("Skipped %d bookmarks older than %s\n", skipped, maxAge)
		}
		bookmarks = filtered
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks to fetch.")
		return
	}

	if limit > 0 && limit < len(bookmarks) {
		bookmarks = bookmarks[:limit]
	}

	fmt.Printf("Fetching %d bookmarks...\n", len(bookmarks))

	fetched := 0
	errors := 0
	skippedUnfetchable := 0
	start := time.Now()

	for i, b := range bookmarks {
		title := b.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Printf("  [%d/%d] %s", i+1, len(bookmarks), title)

		text, err := fetcher.FetchText(b.URL)
		if err != nil {
			errStr := err.Error()
			status := classifyError(errStr)
			fmt.Printf(" - %s\n", status)
			_ = db.UpdateFetchStatus(b.URL, status)
			errors++
			skippedUnfetchable++
			continue
		}

		if text == "" {
			fmt.Printf(" - empty content\n")
			_ = db.UpdateFetchStatus(b.URL, "error:empty")
			errors++
			continue
		}

		if err := db.UpdateContent(b.URL, text); err != nil {
			fmt.Printf(" - DB ERROR: %v\n", err)
			errors++
			continue
		}

		fmt.Printf(" - %d chars\n", len(text))
		fetched++

		if delay > 0 && i < len(bookmarks)-1 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	fmt.Printf("\nDone in %v: %d fetched, %d errors (%d marked unfetchable)\n",
		time.Since(start).Round(time.Millisecond), fetched, errors, skippedUnfetchable)
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

// classifyError returns a status string like "error:404", "error:403", "error:timeout", etc.
func classifyError(errStr string) string {
	if strings.Contains(errStr, "HTTP 404") {
		return "error:404"
	}
	if strings.Contains(errStr, "HTTP 403") {
		return "error:403"
	}
	if strings.Contains(errStr, "HTTP 401") {
		return "error:401"
	}
	if strings.Contains(errStr, "HTTP 410") {
		return "error:410"
	}
	if strings.Contains(errStr, "HTTP 5") {
		return "error:5xx"
	}
	if strings.Contains(errStr, "not HTML") {
		return "error:not-html"
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return "error:timeout"
	}
	if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "DNS") {
		return "error:dns"
	}
	if strings.Contains(errStr, "connection refused") {
		return "error:refused"
	}
	if strings.Contains(errStr, "TLS") || strings.Contains(errStr, "certificate") {
		return "error:tls"
	}
	return "error:" + truncStr(errStr, 50)
}

func truncStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// bookmarkAge returns the best available creation time for the bookmark.
func bookmarkAge(b db.Bookmark) time.Time {
	// Prefer Chrome's original timestamp
	if b.ChromeAddedAt != "" {
		if t, err := time.Parse(time.RFC3339, b.ChromeAddedAt); err == nil {
			return t
		}
	}
	if b.AddedAt != "" {
		if t, err := time.Parse(time.RFC3339, b.AddedAt); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseDuration parses age strings like "1y", "6m", "90d", "2w".
func parseDuration(s string) time.Duration {
	if s == "" || s == "0" {
		return 0
	}
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) < 2 {
		return 0
	}

	numStr := s[:len(s)-1]
	unit := s[len(s)-1]

	var n int
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}

	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour
	case 'm':
		return time.Duration(n) * 30 * 24 * time.Hour
	case 'y':
		return time.Duration(n) * 365 * 24 * time.Hour
	default:
		return 0
	}
}
