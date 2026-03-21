package sync

import (
	"fmt"
	"os"
	"strings"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/spf13/cobra"
)

type Params struct {
	Model   string `long:"model" env:"BM_EMBED_MODEL" help:"Embedding model" default:"qwen3-embedding:0.6b"`
	URL     string `long:"url" env:"BM_OLLAMA_URL" help:"Ollama API base URL" default:"http://localhost:11434"`
	Profile string `short:"p" optional:"true" help:"Filter by profile (name, email, or source ID)"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "sync",
		Short: "Import + fetch + index in one step",
		Long:  "Runs import, fetch, and index sequentially.",
		InitFuncCtx: func(ctx *boa.HookContext, params *Params, cmd *cobra.Command) error {
			ctx.GetParam(&params.Profile).SetAlternativesFunc(profileAlternatives)
			ctx.GetParam(&params.Profile).SetStrictAlts(false)
			return nil
		},
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			setFlag := func(c *cobra.Command, name, value string) {
				if value != "" {
					if err := c.Flags().Set(name, value); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
					}
				}
			}

			fmt.Println("=== Step 1: Import ===")
			for _, c := range cmd.Root().Commands() {
				if c.Name() == "import" {
					setFlag(c, "profile", params.Profile)
					c.Run(c, nil)
					break
				}
			}

			fmt.Println("\n=== Step 2: Fetch ===")
			for _, c := range cmd.Root().Commands() {
				if c.Name() == "fetch" {
					setFlag(c, "profile", params.Profile)
					c.Run(c, nil)
					break
				}
			}

			fmt.Println("\n=== Step 3: Index ===")
			for _, c := range cmd.Root().Commands() {
				if c.Name() == "index" {
					setFlag(c, "model", params.Model)
					setFlag(c, "url", params.URL)
					setFlag(c, "profile", params.Profile)
					c.Run(c, nil)
					break
				}
			}

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
