package chrome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBookmarksFile(t *testing.T) {
	content := `{
		"roots": {
			"bookmark_bar": {
				"type": "folder",
				"name": "Bookmarks Bar",
				"children": [
					{
						"type": "url",
						"name": "Example",
						"url": "https://example.com"
					},
					{
						"type": "folder",
						"name": "Dev",
						"children": [
							{
								"type": "url",
								"name": "Go",
								"url": "https://go.dev"
							},
							{
								"type": "url",
								"name": "Rust",
								"url": "https://rust-lang.org"
							}
						]
					}
				]
			},
			"other": {
				"type": "folder",
				"name": "Other",
				"children": [
					{
						"type": "url",
						"name": "Other Site",
						"url": "https://other.example.com"
					}
				]
			}
		}
	}`

	path := writeTempFile(t, "Bookmarks", content)

	bookmarks, err := ParseBookmarksFile(path)
	if err != nil {
		t.Fatalf("ParseBookmarksFile: %v", err)
	}

	if len(bookmarks) != 4 {
		t.Fatalf("expected 4 bookmarks, got %d", len(bookmarks))
	}

	// Build lookup by URL
	byURL := make(map[string]Bookmark)
	for _, b := range bookmarks {
		byURL[b.URL] = b
	}

	if b, ok := byURL["https://example.com"]; !ok {
		t.Error("missing example.com")
	} else if b.Title != "Example" {
		t.Errorf("expected title 'Example', got %q", b.Title)
	}

	if b, ok := byURL["https://go.dev"]; !ok {
		t.Error("missing go.dev")
	} else if b.FolderPath != "bookmark_bar/Bookmarks Bar/Dev" {
		t.Errorf("unexpected folder path: %q", b.FolderPath)
	}
}

func TestParseBookmarksFile_SkipsNonHTTP(t *testing.T) {
	content := `{
		"roots": {
			"bookmark_bar": {
				"type": "folder",
				"name": "Bar",
				"children": [
					{"type": "url", "name": "JS", "url": "javascript:void(0)"},
					{"type": "url", "name": "Chrome", "url": "chrome://settings"},
					{"type": "url", "name": "Valid", "url": "https://valid.com"}
				]
			}
		}
	}`

	path := writeTempFile(t, "Bookmarks", content)
	bookmarks, err := ParseBookmarksFile(path)
	if err != nil {
		t.Fatalf("ParseBookmarksFile: %v", err)
	}

	if len(bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark (only https), got %d", len(bookmarks))
	}
	if bookmarks[0].URL != "https://valid.com" {
		t.Errorf("expected https://valid.com, got %s", bookmarks[0].URL)
	}
}

func TestParseBookmarksFile_Empty(t *testing.T) {
	content := `{"roots": {}}`
	path := writeTempFile(t, "Bookmarks", content)

	bookmarks, err := ParseBookmarksFile(path)
	if err != nil {
		t.Fatalf("ParseBookmarksFile: %v", err)
	}

	if len(bookmarks) != 0 {
		t.Fatalf("expected 0 bookmarks, got %d", len(bookmarks))
	}
}

func TestParseBookmarksFile_BadJSON(t *testing.T) {
	path := writeTempFile(t, "Bookmarks", "not json")
	_, err := ParseBookmarksFile(path)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestParseBookmarksFile_FileNotFound(t *testing.T) {
	_, err := ParseBookmarksFile("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseBookmarksFile_DeeplyNested(t *testing.T) {
	content := `{
		"roots": {
			"bookmark_bar": {
				"type": "folder",
				"name": "A",
				"children": [{
					"type": "folder",
					"name": "B",
					"children": [{
						"type": "folder",
						"name": "C",
						"children": [{
							"type": "url",
							"name": "Deep",
							"url": "https://deep.example.com"
						}]
					}]
				}]
			}
		}
	}`

	path := writeTempFile(t, "Bookmarks", content)
	bookmarks, err := ParseBookmarksFile(path)
	if err != nil {
		t.Fatalf("ParseBookmarksFile: %v", err)
	}

	if len(bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(bookmarks))
	}
	expected := "bookmark_bar/A/B/C"
	if bookmarks[0].FolderPath != expected {
		t.Errorf("expected folder %q, got %q", expected, bookmarks[0].FolderPath)
	}
}

func TestDiscoverProfiles(t *testing.T) {
	// This test is platform-dependent and only verifies the function doesn't panic.
	// On machines without Chrome, it should return an error or empty list.
	profiles, err := DiscoverProfiles()
	if err != nil {
		t.Skipf("Chrome not installed or unsupported OS: %v", err)
	}
	t.Logf("Found %d Chrome profiles", len(profiles))
	for _, p := range profiles {
		t.Logf("  %s: %s", p.Name, p.Path)
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
