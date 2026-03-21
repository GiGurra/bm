package sync

import (
	"fmt"
	"strings"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/cmd/fetch"
	"github.com/gigurra/bm/cmd/importcmd"
	"github.com/gigurra/bm/cmd/index"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/spf13/cobra"
)

type Params struct {
	Model   string `long:"model" env:"BM_EMBED_MODEL" help:"Embedding model" default:"qwen3-embedding:0.6b"`
	URL     string `long:"url" env:"BM_OLLAMA_URL" help:"Ollama API base URL" default:"http://localhost:11434"`
	Profile string `short:"p" optional:"true" help:"Filter by profile (name, email, or source ID)"`
	MaxAge  string `long:"max-age" help:"Skip bookmarks older than this (e.g. 1y, 6m, 90d)" default:"1y"`
	Fetch   bool   `long:"fetch" help:"Also fetch page content (beta)"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "sync",
		Short: "Import + index in one step",
		Long:  "Runs import and index sequentially. Use --fetch to also fetch page content (beta).",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(profileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			fmt.Println("=== Step 1: Import ===")
			importcmd.Run("", params.Profile)

			if params.Fetch {
				fmt.Println("\n=== Step 2: Fetch ===")
				fetch.Run(params.Profile, params.MaxAge, false, 0, 500)
			}

			step := 2
			if params.Fetch {
				step = 3
			}
			fmt.Printf("\n=== Step %d: Index ===\n", step)
			index.Run(params.URL, params.Model, params.Profile, params.MaxAge, false)

			fmt.Println("\nSync complete!")
		},
	}.ToCobra()
}

func profileAlternatives(_ *cobra.Command, _ []string, toComplete string) []string {
	profiles, err := chrome.DiscoverProfiles()
	if err != nil {
		return nil
	}
	var alts []string
	for _, p := range profiles {
		for _, candidate := range []string{p.UserName, p.SourceID(), p.DirName} {
			if candidate != "" && strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(toComplete)) {
				alts = append(alts, candidate)
			}
		}
	}
	return alts
}
