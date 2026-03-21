package db

import (
	"testing"
	"time"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	_, err := OpenMem()
	if err != nil {
		t.Fatalf("OpenMem: %v", err)
	}
}

func TestUpsertAndListBookmarks(t *testing.T) {
	setupTestDB(t)

	err := UpsertBookmark(&Bookmark{
		URL:        "https://example.com",
		Title:      "Example",
		FolderPath: "bar/folder",
		Source:     "chrome",
	})
	if err != nil {
		t.Fatalf("UpsertBookmark: %v", err)
	}

	err = UpsertBookmark(&Bookmark{
		URL:        "https://go.dev",
		Title:      "Go",
		FolderPath: "bar/dev",
		Source:     "chrome",
	})
	if err != nil {
		t.Fatalf("UpsertBookmark: %v", err)
	}

	bookmarks, err := ListBookmarks()
	if err != nil {
		t.Fatalf("ListBookmarks: %v", err)
	}
	if len(bookmarks) != 2 {
		t.Fatalf("expected 2, got %d", len(bookmarks))
	}
}

func TestUpsertBookmark_UpdatesTitle(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "Old Title", Source: "chrome"})
	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "New Title", Source: "chrome"})

	bookmarks, _ := ListBookmarks()
	if len(bookmarks) != 1 {
		t.Fatalf("expected 1 (upsert), got %d", len(bookmarks))
	}
	if bookmarks[0].Title != "New Title" {
		t.Errorf("expected 'New Title', got %q", bookmarks[0].Title)
	}
}

func TestUpsertBookmark_PreservesContent(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "Example", Source: "chrome"})
	_ = UpdateContent("https://example.com", "page content here")

	// Re-import should NOT overwrite content
	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "Example Updated", Source: "chrome"})

	bookmarks, _ := ListBookmarks()
	if bookmarks[0].ContentText != "page content here" {
		t.Errorf("content was overwritten: %q", bookmarks[0].ContentText)
	}
	if bookmarks[0].Title != "Example Updated" {
		t.Errorf("title not updated: %q", bookmarks[0].Title)
	}
}

func TestUpdateContent(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "Example", Source: "chrome"})
	_ = UpdateContent("https://example.com", "hello world content")

	bookmarks, _ := ListBookmarks()
	if bookmarks[0].ContentText != "hello world content" {
		t.Errorf("unexpected content: %q", bookmarks[0].ContentText)
	}
	if bookmarks[0].FetchedAt == "" {
		t.Error("fetched_at should be set")
	}
}

func TestListFetchable(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://a.com", Title: "A", Source: "chrome"})
	_ = UpsertBookmark(&Bookmark{URL: "https://b.com", Title: "B", Source: "chrome"})
	_ = UpsertBookmark(&Bookmark{URL: "https://c.com", Title: "C", Source: "chrome"})
	_ = UpdateContent("https://a.com", "fetched content")
	_ = UpdateFetchStatus("https://c.com", "error:404")

	fetchable, err := ListFetchable()
	if err != nil {
		t.Fatalf("ListFetchable: %v", err)
	}
	if len(fetchable) != 1 {
		t.Fatalf("expected 1 fetchable, got %d", len(fetchable))
	}
	if fetchable[0].URL != "https://b.com" {
		t.Errorf("expected b.com, got %s", fetchable[0].URL)
	}
}

func TestUpdateFetchStatus(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://gone.com", Title: "Gone", Source: "chrome"})
	_ = UpdateFetchStatus("https://gone.com", "error:404")

	bookmarks, _ := ListBookmarks()
	if bookmarks[0].FetchStatus != "error:404" {
		t.Errorf("expected 'error:404', got %q", bookmarks[0].FetchStatus)
	}
	if bookmarks[0].FetchedAt == "" {
		t.Error("fetched_at should be set even for errors")
	}
}

func TestChromeAddedAt(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "Ex", Source: "chrome", ChromeAddedAt: "2020-01-15T10:30:00Z"})

	bookmarks, _ := ListBookmarks()
	if bookmarks[0].ChromeAddedAt != "2020-01-15T10:30:00Z" {
		t.Errorf("expected chrome timestamp, got %q", bookmarks[0].ChromeAddedAt)
	}

	// Re-import without chrome timestamp should preserve the existing one
	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "Ex Updated", Source: "chrome"})
	bookmarks, _ = ListBookmarks()
	if bookmarks[0].ChromeAddedAt != "2020-01-15T10:30:00Z" {
		t.Errorf("chrome timestamp was overwritten: %q", bookmarks[0].ChromeAddedAt)
	}
}

func TestSearchFTS(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://go.dev", Title: "The Go Programming Language", Source: "chrome"})
	_ = UpsertBookmark(&Bookmark{URL: "https://rust-lang.org", Title: "Rust Programming Language", Source: "chrome"})
	_ = UpdateContent("https://go.dev", "Go is an open source programming language for building systems")

	results, err := SearchFTS("Go programming", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].URL != "https://go.dev" {
		t.Errorf("expected go.dev as top result, got %s", results[0].URL)
	}

	// Search in content
	results, err = SearchFTS("open source systems", 10)
	if err != nil {
		t.Fatalf("SearchFTS content: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected result from content search")
	}
}

func TestSearchFTS_NoResults(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://example.com", Title: "Example", Source: "chrome"})

	results, err := SearchFTS("nonexistent_xyzzy", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestCountBookmarks(t *testing.T) {
	setupTestDB(t)

	_ = UpsertBookmark(&Bookmark{URL: "https://a.com", Title: "A", Source: "chrome"})
	_ = UpsertBookmark(&Bookmark{URL: "https://b.com", Title: "B", Source: "chrome"})

	count, err := CountBookmarks()
	if err != nil {
		t.Fatalf("CountBookmarks: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}

	fetched, err := CountFetched()
	if err != nil {
		t.Fatalf("CountFetched: %v", err)
	}
	if fetched != 0 {
		t.Errorf("expected 0 fetched, got %d", fetched)
	}

	_ = UpdateContent("https://a.com", "content")
	fetched, _ = CountFetched()
	if fetched != 1 {
		t.Errorf("expected 1 fetched, got %d", fetched)
	}
}

func TestBulkUpsert_FreshDB(t *testing.T) {
	setupTestDB(t)

	bookmarks := []*Bookmark{
		{URL: "https://a.com", Title: "A", FolderPath: "bar", Source: "chrome", ChromeAddedAt: "2020-01-01T00:00:00Z"},
		{URL: "https://b.com", Title: "B", FolderPath: "bar", Source: "chrome", ChromeAddedAt: "2020-02-01T00:00:00Z"},
		{URL: "https://c.com", Title: "C", FolderPath: "other", Source: "chrome", ChromeAddedAt: "2020-03-01T00:00:00Z"},
	}

	inserted, updated, total, err := BulkUpsertBookmarks(bookmarks)
	if err != nil {
		t.Fatalf("BulkUpsertBookmarks: %v", err)
	}
	if inserted != 3 || updated != 0 || total != 3 {
		t.Errorf("expected 3/0/3, got %d/%d/%d", inserted, updated, total)
	}

	all, _ := ListBookmarks()
	if len(all) != 3 {
		t.Fatalf("expected 3 in DB, got %d", len(all))
	}
}

func TestBulkUpsert_Idempotent(t *testing.T) {
	setupTestDB(t)

	bookmarks := []*Bookmark{
		{URL: "https://a.com", Title: "A", FolderPath: "bar", Source: "chrome", ChromeAddedAt: "2020-01-01T00:00:00Z"},
		{URL: "https://b.com", Title: "B", FolderPath: "bar", Source: "chrome", ChromeAddedAt: "2020-02-01T00:00:00Z"},
	}

	BulkUpsertBookmarks(bookmarks)

	// Second import with same data
	inserted, updated, total, err := BulkUpsertBookmarks(bookmarks)
	if err != nil {
		t.Fatalf("BulkUpsertBookmarks: %v", err)
	}
	if inserted != 0 || updated != 0 || total != 2 {
		t.Errorf("expected 0/0/2, got %d/%d/%d", inserted, updated, total)
	}
}

func TestBulkUpsert_DetectsChanges(t *testing.T) {
	setupTestDB(t)

	bookmarks := []*Bookmark{
		{URL: "https://a.com", Title: "A", FolderPath: "bar", Source: "chrome"},
		{URL: "https://b.com", Title: "B", FolderPath: "bar", Source: "chrome"},
	}
	BulkUpsertBookmarks(bookmarks)

	// Change title of one
	bookmarks[0].Title = "A Updated"

	inserted, updated, total, err := BulkUpsertBookmarks(bookmarks)
	if err != nil {
		t.Fatalf("BulkUpsertBookmarks: %v", err)
	}
	if inserted != 0 || updated != 1 || total != 2 {
		t.Errorf("expected 0/1/2, got %d/%d/%d", inserted, updated, total)
	}
}

func TestBulkUpsert_CompositeKey(t *testing.T) {
	setupTestDB(t)

	// Same URL in different folders = different entries
	bookmarks := []*Bookmark{
		{URL: "https://a.com", Title: "A in bar", FolderPath: "bar", Source: "chrome"},
		{URL: "https://a.com", Title: "A in other", FolderPath: "other", Source: "chrome"},
	}

	inserted, updated, total, err := BulkUpsertBookmarks(bookmarks)
	if err != nil {
		t.Fatalf("BulkUpsertBookmarks: %v", err)
	}
	if inserted != 2 || updated != 0 || total != 2 {
		t.Errorf("expected 2/0/2, got %d/%d/%d", inserted, updated, total)
	}

	all, _ := ListBookmarks()
	if len(all) != 2 {
		t.Fatalf("expected 2 in DB, got %d", len(all))
	}
}

func TestBulkUpsert_DedupKeepsLatest(t *testing.T) {
	setupTestDB(t)

	// Duplicate composite key — should keep the one with latest chrome_added_at
	bookmarks := []*Bookmark{
		{URL: "https://a.com", Title: "Old Title", FolderPath: "bar", Source: "chrome", ChromeAddedAt: "2020-01-01T00:00:00Z"},
		{URL: "https://a.com", Title: "New Title", FolderPath: "bar", Source: "chrome", ChromeAddedAt: "2022-06-15T00:00:00Z"},
		{URL: "https://b.com", Title: "B", FolderPath: "bar", Source: "chrome", ChromeAddedAt: "2021-01-01T00:00:00Z"},
	}

	inserted, _, total, err := BulkUpsertBookmarks(bookmarks)
	if err != nil {
		t.Fatalf("BulkUpsertBookmarks: %v", err)
	}
	if inserted != 2 || total != 2 {
		t.Errorf("expected 2 inserted, 2 total, got %d/%d", inserted, total)
	}

	all, _ := ListBookmarks()
	if len(all) != 2 {
		t.Fatalf("expected 2 in DB, got %d", len(all))
	}

	// Verify the latest one won
	for _, b := range all {
		if b.URL == "https://a.com" {
			if b.Title != "New Title" {
				t.Errorf("expected 'New Title' (latest), got %q", b.Title)
			}
			if b.ChromeAddedAt != "2022-06-15T00:00:00Z" {
				t.Errorf("expected latest chrome_added_at, got %q", b.ChromeAddedAt)
			}
		}
	}
}

func TestBulkUpsert_PreservesContent(t *testing.T) {
	setupTestDB(t)

	bookmarks := []*Bookmark{
		{URL: "https://a.com", Title: "A", FolderPath: "bar", Source: "chrome"},
	}
	BulkUpsertBookmarks(bookmarks)
	UpdateContent("https://a.com", "fetched page content")

	// Re-import should not overwrite content
	bookmarks[0].Title = "A Updated"
	BulkUpsertBookmarks(bookmarks)

	all, _ := ListBookmarks()
	if all[0].ContentText != "fetched page content" {
		t.Errorf("content was overwritten: %q", all[0].ContentText)
	}
	if all[0].Title != "A Updated" {
		t.Errorf("title not updated: %q", all[0].Title)
	}
}

func TestBulkUpsert_MultiSource(t *testing.T) {
	setupTestDB(t)

	// Same URL+folder from different sources = different entries
	bookmarks := []*Bookmark{
		{URL: "https://a.com", Title: "A", FolderPath: "bar", Source: "chrome:profile1"},
		{URL: "https://a.com", Title: "A", FolderPath: "bar", Source: "chrome:profile2"},
	}

	inserted, updated, total, err := BulkUpsertBookmarks(bookmarks)
	if err != nil {
		t.Fatalf("BulkUpsertBookmarks: %v", err)
	}
	if inserted != 2 || updated != 0 || total != 2 {
		t.Errorf("expected 2/0/2, got %d/%d/%d", inserted, updated, total)
	}
}

func TestEmbeddingCRUD(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	row := &EmbeddingRow{
		URL:        "https://example.com",
		ChunkIndex: 0,
		ChunkText:  "test chunk",
		Embedding:  []byte{1, 2, 3, 4},
		Model:      "test-model",
		CreatedAt:  now,
	}

	if err := UpsertEmbedding(row); err != nil {
		t.Fatalf("UpsertEmbedding: %v", err)
	}

	// List all
	all, err := ListAllEmbeddings()
	if err != nil {
		t.Fatalf("ListAllEmbeddings: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}
	if all[0].ChunkText != "test chunk" {
		t.Errorf("unexpected chunk text: %q", all[0].ChunkText)
	}

	// Upsert same key — should update
	row.ChunkText = "updated chunk"
	_ = UpsertEmbedding(row)
	all, _ = ListAllEmbeddings()
	if len(all) != 1 {
		t.Fatalf("expected 1 after upsert, got %d", len(all))
	}
	if all[0].ChunkText != "updated chunk" {
		t.Errorf("upsert didn't update: %q", all[0].ChunkText)
	}

	// List embedded URLs
	urls, err := ListEmbeddedURLs()
	if err != nil {
		t.Fatalf("ListEmbeddedURLs: %v", err)
	}
	if _, ok := urls["https://example.com"]; !ok {
		t.Error("expected example.com in embedded URLs")
	}

	// List models
	models, err := ListEmbeddingModels()
	if err != nil {
		t.Fatalf("ListEmbeddingModels: %v", err)
	}
	if len(models) != 1 || models[0] != "test-model" {
		t.Errorf("unexpected models: %v", models)
	}

	// Delete
	if err := DeleteEmbeddingsForURL("https://example.com"); err != nil {
		t.Fatalf("DeleteEmbeddingsForURL: %v", err)
	}
	all, _ = ListAllEmbeddings()
	if len(all) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(all))
	}
}
