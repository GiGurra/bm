package index

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/cmd/common"
	"github.com/gigurra/bm/pkg/config"
	"github.com/gigurra/bm/pkg/db"
	"github.com/gigurra/bm/pkg/ollama"
	"github.com/spf13/cobra"
)

const maxChunkChars = 24000

type Params struct {
	Reindex bool   `long:"reindex" help:"Force re-index all bookmarks"`
	Model   string `long:"model" env:"BM_EMBED_MODEL" help:"Embedding model" default:"qwen3-embedding:0.6b"`
	URL     string `long:"url" env:"BM_OLLAMA_URL" help:"Ollama API base URL" default:"http://localhost:11434"`
	MaxAge  string `long:"max-age" help:"Skip bookmarks older than this (e.g. 1y, 6m, 90d)" default:"1y"`
	Profile string `short:"p" optional:"true" env:"BM_PROFILE" help:"Filter by profile (name, email, or source ID; 'all' for all profiles)"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "index",
		Short: "Build semantic search index using local embeddings",
		Long:  "Generates embeddings via Ollama for bookmarks that have fetched content.",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(common.ProfileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			Run(params.URL, params.Model, params.Profile, params.MaxAge, params.Reindex)
		},
	}.ToCobra()
}

func Run(url, model, profile, maxAge string, reindex bool) {
	client := ollama.NewClient(url, model)

	// Test connection
	if _, err := client.EmbedOne("test"); err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to Ollama: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nMake sure Ollama is running:\n")
		fmt.Fprintf(os.Stderr, "  brew services start ollama\n")
		fmt.Fprintf(os.Stderr, "  ollama pull %s\n", model)
		os.Exit(1)
	}

	sources, err := config.ResolveSourceIDs(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var bookmarks []db.Bookmark
	bookmarks, err = db.ListBookmarksBySources(sources)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks. Run 'bm import' first.")
		return
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

	// Check what's already indexed
	embeddedKeys, err := db.ListEmbeddedKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if reindex {
		embeddedKeys = make(map[db.BookmarkKey]db.EmbeddedInfo)
	}

	var toIndex []db.Bookmark
	changed := 0
	for _, b := range bookmarks {
		key := db.BookmarkKey{URL: b.URL, FolderPath: b.FolderPath, Source: b.Source}
		info, exists := embeddedKeys[key]
		if !exists {
			toIndex = append(toIndex, b)
		} else if info.ContentHash == "" || info.ContentHash != contentHash(b) {
			// Empty hash (legacy) or content changed — re-index
			toIndex = append(toIndex, b)
			changed++
		}
	}

	if len(toIndex) == 0 {
		fmt.Printf("All %d bookmarks already indexed.\n", len(bookmarks))
		return
	}

	if changed > 0 {
		fmt.Printf("Indexing %d bookmarks (%d new, %d changed, %d up-to-date)...\n",
			len(toIndex), len(toIndex)-changed, changed, len(bookmarks)-len(toIndex))
	} else {
		fmt.Printf("Indexing %d bookmarks (%d already indexed)...\n",
			len(toIndex), len(bookmarks)-len(toIndex))
	}

	start := time.Now()
	totalChunks := 0
	errors := 0

	for i, b := range toIndex {
		title := b.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Printf("  [%d/%d] %s", i+1, len(toIndex), title)

		chunks := chunkBookmark(b)
		hash := contentHash(b)

		// Delete old embeddings when re-indexing (forced, changed, or legacy without hash)
		key := db.BookmarkKey{URL: b.URL, FolderPath: b.FolderPath, Source: b.Source}
		if _, exists := embeddedKeys[key]; reindex || exists {
			_ = db.DeleteEmbeddings(b.URL, b.FolderPath, b.Source)
		}

		stored := 0
		for _, chunk := range chunks {
			embedding, err := client.EmbedOne(chunk.text)
			if err != nil {
				continue
			}
			row := &db.EmbeddingRow{
				URL:         b.URL,
				FolderPath:  b.FolderPath,
				Source:      b.Source,
				ChunkIndex:  chunk.index,
				ChunkText:   chunk.text,
				Embedding:   ollama.Float32ToBytes(embedding),
				Model:       client.Model,
				CreatedAt:   time.Now(),
				ContentHash: hash,
			}
			if err := db.UpsertEmbedding(row); err != nil {
				fmt.Printf(" - DB ERROR: %v\n", err)
				errors++
				continue
			}
			stored++
		}
		totalChunks += stored
		fmt.Printf(" - %d chunks\n", stored)
	}

	fmt.Printf("\nDone in %v: %d bookmarks, %d chunks, %d errors\n",
		time.Since(start).Round(time.Millisecond), len(toIndex)-errors, totalChunks, errors)
}

type chunk struct {
	index int
	text  string
}

func chunkBookmark(b db.Bookmark) []chunk {
	var chunks []chunk

	// Chunk 0: metadata (title + URL + folder)
	meta := fmt.Sprintf("Title: %s\nURL: %s\nFolder: %s", b.Title, b.URL, b.FolderPath)
	if len(meta) > maxChunkChars {
		meta = meta[:maxChunkChars]
	}
	chunks = append(chunks, chunk{index: 0, text: meta})

	// Chunk 1+: content in maxChunkChars slices
	content := b.ContentText
	idx := 1
	for len(content) > 0 {
		end := maxChunkChars
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, chunk{index: idx, text: content[:end]})
		content = content[end:]
		idx++
	}

	return chunks
}

func bookmarkAge(b db.Bookmark) time.Time {
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

// contentHash returns a SHA-256 hex digest of the bookmark fields used for embedding.
// A change in any of these fields means the embeddings are stale.
func contentHash(b db.Bookmark) string {
	h := sha256.New()
	h.Write([]byte(b.Title))
	h.Write([]byte{0})
	h.Write([]byte(b.URL))
	h.Write([]byte{0})
	h.Write([]byte(b.FolderPath))
	h.Write([]byte{0})
	h.Write([]byte(b.ContentText))
	return fmt.Sprintf("%x", h.Sum(nil))
}
