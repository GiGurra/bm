package chrome

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Bookmark represents a single Chrome bookmark.
type Bookmark struct {
	URL        string
	Title      string
	FolderPath string
	DateAdded  string // RFC3339 timestamp from Chrome's date_added field
}

// Profile represents a Chrome profile with its bookmark file path and identity info.
type Profile struct {
	DirName  string // directory name (e.g. "Default", "Profile 1") — unstable, Chrome can reassign
	Path     string // full path to Bookmarks JSON file
	GaiaID   string // stable Google account ID
	UserName string // Google account email
	Name     string // profile display name (usually same as email)
}

// SourceID returns a stable identifier for this profile suitable for the DB source field.
// Prefers gaia_id (stable across profile directory renames), falls back to directory name.
func (p Profile) SourceID() string {
	if p.GaiaID != "" {
		return "chrome:gaia:" + p.GaiaID
	}
	return "chrome:" + p.DirName
}

// DisplayName returns a human-readable label for this profile.
func (p Profile) DisplayName() string {
	if p.UserName != "" {
		return fmt.Sprintf("%s (%s)", p.UserName, p.DirName)
	}
	return p.DirName
}

// chromeBaseDir returns the Chrome user data directory for the current OS.
func chromeBaseDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	case "linux":
		return filepath.Join(home, ".config", "google-chrome")
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "User Data")
	default:
		return ""
	}
}

// profileInfoCache represents the profile metadata from Chrome's "Local State" file.
type profileInfoCache struct {
	Name     string `json:"name"`
	GaiaID   string `json:"gaia_id"`
	UserName string `json:"user_name"`
}

// loadProfileInfo reads Chrome's "Local State" JSON and returns profile metadata keyed by directory name.
func loadProfileInfo(base string) map[string]profileInfoCache {
	data, err := os.ReadFile(filepath.Join(base, "Local State"))
	if err != nil {
		return nil
	}

	var localState struct {
		Profile struct {
			InfoCache map[string]profileInfoCache `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &localState); err != nil {
		return nil
	}

	return localState.Profile.InfoCache
}

// DiscoverProfiles finds all Chrome profiles that have a Bookmarks file,
// enriched with stable identity info from Chrome's Local State.
func DiscoverProfiles() ([]Profile, error) {
	base := chromeBaseDir()
	if base == "" {
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("read Chrome directory: %w", err)
	}

	infoCache := loadProfileInfo(base)

	var profiles []Profile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		if dirName != "Default" && !strings.HasPrefix(dirName, "Profile ") {
			continue
		}
		bmPath := filepath.Join(base, dirName, "Bookmarks")
		if _, err := os.Stat(bmPath); err != nil {
			continue
		}

		p := Profile{
			DirName: dirName,
			Path:    bmPath,
		}
		if info, ok := infoCache[dirName]; ok {
			p.GaiaID = info.GaiaID
			p.UserName = info.UserName
			p.Name = info.Name
		}
		profiles = append(profiles, p)
	}

	return profiles, nil
}

// DefaultBookmarksPath returns the Chrome Bookmarks JSON file path for the default profile.
func DefaultBookmarksPath() string {
	base := chromeBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Default", "Bookmarks")
}

type chromeFile struct {
	Roots map[string]json.RawMessage `json:"roots"`
}

type chromeNode struct {
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	URL       string          `json:"url,omitempty"`
	DateAdded string          `json:"date_added,omitempty"` // Chrome WebKit timestamp (microseconds since 1601-01-01)
	Children  json.RawMessage `json:"children,omitempty"`
}

// ParseBookmarksFile reads a Chrome Bookmarks JSON file and returns all bookmarks.
func ParseBookmarksFile(path string) ([]Bookmark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bookmarks file: %w", err)
	}

	var file chromeFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse bookmarks JSON: %w", err)
	}

	var bookmarks []Bookmark
	for rootName, raw := range file.Roots {
		var node chromeNode
		if err := json.Unmarshal(raw, &node); err != nil {
			continue
		}
		walkNode(node, rootName, &bookmarks)
	}

	return bookmarks, nil
}

// chromeTimeToRFC3339 converts Chrome's WebKit timestamp (microseconds since 1601-01-01)
// to an RFC3339 string.
func chromeTimeToRFC3339(chromeTimestamp string) string {
	usec, err := strconv.ParseInt(chromeTimestamp, 10, 64)
	if err != nil || usec == 0 {
		return ""
	}
	// Chrome epoch is 1601-01-01. Unix epoch is 1970-01-01.
	// Difference: 11644473600 seconds.
	const chromeToUnixDelta = 11644473600
	unixSec := usec/1_000_000 - chromeToUnixDelta
	if unixSec < 0 {
		return ""
	}
	return time.Unix(unixSec, (usec%1_000_000)*1000).Format(time.RFC3339)
}

// Dedup removes duplicate bookmarks by (URL, FolderPath), keeping the entry
// with the latest DateAdded. This matches the dedup logic in BulkUpsertBookmarks.
func Dedup(bookmarks []Bookmark) []Bookmark {
	type key struct{ url, folder string }
	deduped := make(map[key]Bookmark, len(bookmarks))
	for _, b := range bookmarks {
		k := key{b.URL, b.FolderPath}
		if prev, exists := deduped[k]; !exists || b.DateAdded > prev.DateAdded {
			deduped[k] = b
		}
	}
	result := make([]Bookmark, 0, len(deduped))
	for _, b := range deduped {
		result = append(result, b)
	}
	return result
}

func walkNode(node chromeNode, folderPath string, out *[]Bookmark) {
	if node.Type == "url" && node.URL != "" {
		// Skip non-http bookmarks (javascript:, chrome:, etc.)
		if strings.HasPrefix(node.URL, "http://") || strings.HasPrefix(node.URL, "https://") {
			*out = append(*out, Bookmark{
				URL:        node.URL,
				Title:      node.Name,
				FolderPath: folderPath,
				DateAdded:  chromeTimeToRFC3339(node.DateAdded),
			})
		}
		return
	}

	if node.Children == nil {
		return
	}

	var children []chromeNode
	if err := json.Unmarshal(node.Children, &children); err != nil {
		return
	}

	childPath := folderPath
	if node.Name != "" {
		childPath = folderPath + "/" + node.Name
	}

	for _, child := range children {
		walkNode(child, childPath, out)
	}
}
