package index

import (
	"testing"

	"github.com/gigurra/bm/pkg/db"
)

func TestContentHash_Deterministic(t *testing.T) {
	b := db.Bookmark{Title: "Go", URL: "https://go.dev", FolderPath: "dev", ContentText: "content"}
	h1 := contentHash(b)
	h2 := contentHash(b)
	if h1 != h2 {
		t.Errorf("expected same hash, got %q and %q", h1, h2)
	}
}

func TestContentHash_ChangesOnTitleChange(t *testing.T) {
	b := db.Bookmark{Title: "Go", URL: "https://go.dev", FolderPath: "dev", ContentText: "content"}
	h1 := contentHash(b)
	b.Title = "Golang"
	h2 := contentHash(b)
	if h1 == h2 {
		t.Error("expected different hash when title changes")
	}
}

func TestContentHash_ChangesOnContentChange(t *testing.T) {
	b := db.Bookmark{Title: "Go", URL: "https://go.dev", FolderPath: "dev", ContentText: "content"}
	h1 := contentHash(b)
	b.ContentText = "new content"
	h2 := contentHash(b)
	if h1 == h2 {
		t.Error("expected different hash when content changes")
	}
}

func TestContentHash_ChangesOnFolderChange(t *testing.T) {
	b := db.Bookmark{Title: "Go", URL: "https://go.dev", FolderPath: "dev", ContentText: "content"}
	h1 := contentHash(b)
	b.FolderPath = "other/dev"
	h2 := contentHash(b)
	if h1 == h2 {
		t.Error("expected different hash when folder changes")
	}
}

func TestContentHash_NullSeparatorPreventsCollision(t *testing.T) {
	// Without null separators, "ab" + "cd" would hash the same as "a" + "bcd"
	b1 := db.Bookmark{Title: "ab", URL: "cd", FolderPath: "", ContentText: ""}
	b2 := db.Bookmark{Title: "a", URL: "bcd", FolderPath: "", ContentText: ""}
	h1 := contentHash(b1)
	h2 := contentHash(b2)
	if h1 == h2 {
		t.Error("expected different hashes — null separator should prevent collision")
	}
}
