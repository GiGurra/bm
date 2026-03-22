package search

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/cmd/common"
	"github.com/gigurra/bm/pkg/db"
	"github.com/gigurra/bm/pkg/ollama"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type Params struct {
	Query    string `pos:"true" help:"Search query"`
	Limit    int    `short:"n" help:"Max results" default:"10"`
	Semantic bool   `short:"s" help:"Use semantic search (requires index + Ollama)"`
	Profile  string `short:"p" optional:"true" env:"BM_PROFILE" help:"Filter by profile (name, email, or source ID; 'all' for all profiles)"`
	Model    string `long:"model" env:"BM_EMBED_MODEL" help:"Embedding model" default:"qwen3-embedding:0.6b"`
	URL      string `long:"url" env:"BM_OLLAMA_URL" help:"Ollama API base URL" default:"http://localhost:11434"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "search",
		Short: "Search bookmarks by text or meaning",
		Long:  "Text search (FTS5) by default, or semantic search with -s flag.",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(common.ProfileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			if params.Query == "" {
				fmt.Fprintln(os.Stderr, "Query is required")
				os.Exit(1)
			}

			if params.Semantic {
				runSemantic(params)
			} else {
				runFTS(params)
			}
		},
	}.ToCobra()
}

func runFTS(params *Params) {
	results, err := db.SearchFTS(params.Query, params.Limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.SetAllowedRowLength(getTerminalWidth())
	t.AppendHeader(table.Row{"#", "Title", "URL", "Folder"})

	for i, b := range results {
		t.AppendRow(table.Row{i + 1, b.Title, b.URL, b.FolderPath})
	}

	t.AppendFooter(table.Row{"", fmt.Sprintf("%d result(s)", len(results)), "", ""})
	t.Render()
}

func runSemantic(params *Params) {
	client := ollama.NewClient(params.URL, params.Model)

	// Check model mismatch
	if models, err := db.ListEmbeddingModels(); err == nil && len(models) > 0 {
		for _, m := range models {
			if m != params.Model {
				fmt.Fprintf(os.Stderr, "Error: index built with %q, searching with %q\n", m, params.Model)
				fmt.Fprintf(os.Stderr, "Run 'bm index --reindex' to rebuild\n")
				os.Exit(1)
			}
		}
	}

	queryEmbedding, err := client.EmbedOne(params.Query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
		os.Exit(1)
	}

	allEmbeddings, err := db.ListAllEmbeddings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(allEmbeddings) == 0 {
		fmt.Println("No embeddings found. Run 'bm index' first.")
		return
	}

	// Build bookmark lookup
	bookmarks, err := db.ListBookmarks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	byKey := make(map[db.BookmarkKey]db.Bookmark, len(bookmarks))
	for _, b := range bookmarks {
		byKey[db.BookmarkKey{URL: b.URL, FolderPath: b.FolderPath, Source: b.Source}] = b
	}

	// Find best-matching chunk per bookmark key
	type match struct {
		key        db.BookmarkKey
		similarity float32
	}
	bestByKey := make(map[db.BookmarkKey]*match)

	for _, emb := range allEmbeddings {
		vec := ollama.BytesToFloat32(emb.Embedding)
		sim := ollama.CosineSimilarity(queryEmbedding, vec)

		key := db.BookmarkKey{URL: emb.URL, FolderPath: emb.FolderPath, Source: emb.Source}
		if best, ok := bestByKey[key]; !ok || sim > best.similarity {
			bestByKey[key] = &match{key: key, similarity: sim}
		}
	}

	// Sort by similarity
	type result struct {
		bookmark   db.Bookmark
		similarity float32
	}
	var results []result
	for _, m := range bestByKey {
		if b, ok := byKey[m.key]; ok {
			results = append(results, result{bookmark: b, similarity: m.similarity})
		}
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].similarity > results[i].similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if params.Limit > 0 && params.Limit < len(results) {
		results = results[:params.Limit]
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.SetAllowedRowLength(getTerminalWidth())
	t.AppendHeader(table.Row{"#", "Score", "Title", "URL", "Folder"})

	for i, r := range results {
		t.AppendRow(table.Row{i + 1, fmt.Sprintf("%.3f", r.similarity), r.bookmark.Title, r.bookmark.URL, r.bookmark.FolderPath})
	}

	t.AppendFooter(table.Row{"", "", fmt.Sprintf("%d result(s)", len(results)), "", ""})
	t.Render()
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
