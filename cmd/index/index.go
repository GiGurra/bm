package index

import (
	"fmt"
	"os"
	"time"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/db"
	"github.com/gigurra/bm/pkg/ollama"
	"github.com/spf13/cobra"
)

const maxChunkChars = 24000

type Params struct {
	Reindex bool   `long:"reindex" help:"Force re-index all bookmarks"`
	Model   string `long:"model" env:"BM_EMBED_MODEL" help:"Embedding model" default:"qwen3-embedding:0.6b"`
	URL     string `long:"url" env:"BM_OLLAMA_URL" help:"Ollama API base URL" default:"http://localhost:11434"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "index",
		Short: "Build semantic search index using local embeddings",
		Long:  "Generates embeddings via Ollama for bookmarks that have fetched content.",
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			client := ollama.NewClient(params.URL, params.Model)

			// Test connection
			if _, err := client.EmbedOne("test"); err != nil {
				fmt.Fprintf(os.Stderr, "Error connecting to Ollama: %v\n", err)
				fmt.Fprintf(os.Stderr, "\nMake sure Ollama is running:\n")
				fmt.Fprintf(os.Stderr, "  brew services start ollama\n")
				fmt.Fprintf(os.Stderr, "  ollama pull %s\n", params.Model)
				os.Exit(1)
			}

			bookmarks, err := db.ListBookmarks()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(bookmarks) == 0 {
				fmt.Println("No bookmarks. Run 'bm import' first.")
				return
			}

			// Check what's already indexed
			embeddedURLs, err := db.ListEmbeddedURLs()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if params.Reindex {
				embeddedURLs = make(map[string]time.Time)
			}

			var toIndex []db.Bookmark
			for _, b := range bookmarks {
				if _, exists := embeddedURLs[b.URL]; !exists {
					toIndex = append(toIndex, b)
				}
			}

			if len(toIndex) == 0 {
				fmt.Printf("All %d bookmarks already indexed.\n", len(bookmarks))
				return
			}

			fmt.Printf("Indexing %d bookmarks (%d already indexed)...\n",
				len(toIndex), len(bookmarks)-len(toIndex))

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
				if params.Reindex {
					_ = db.DeleteEmbeddingsForURL(b.URL)
				}

				stored := 0
				for _, chunk := range chunks {
					embedding, err := client.EmbedOne(chunk.text)
					if err != nil {
						continue
					}
					row := &db.EmbeddingRow{
						URL:        b.URL,
						ChunkIndex: chunk.index,
						ChunkText:  chunk.text,
						Embedding:  ollama.Float32ToBytes(embedding),
						Model:      client.Model,
						CreatedAt:  time.Now(),
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
		},
	}.ToCobra()
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
