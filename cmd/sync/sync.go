package sync

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/spf13/cobra"
)

type Params struct {
	Model string `long:"model" env:"BM_EMBED_MODEL" help:"Embedding model" default:"qwen3-embedding:0.6b"`
	URL   string `long:"url" env:"BM_OLLAMA_URL" help:"Ollama API base URL" default:"http://localhost:11434"`
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:   "sync",
		Short: "Import + fetch + index in one step",
		Long:  "Runs import, fetch, and index sequentially.",
		RunFunc: func(params *Params, cmd *cobra.Command, args []string) {
			fmt.Println("=== Step 1: Import ===")
			importCmd := cmd.Root().Commands()
			for _, c := range importCmd {
				if c.Name() == "import" {
					c.Run(c, nil)
					break
				}
			}

			fmt.Println("\n=== Step 2: Fetch ===")
			for _, c := range importCmd {
				if c.Name() == "fetch" {
					c.Run(c, nil)
					break
				}
			}

			fmt.Println("\n=== Step 3: Index ===")
			for _, c := range importCmd {
				if c.Name() == "index" {
					if err := c.Flags().Set("model", params.Model); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
					}
					if err := c.Flags().Set("url", params.URL); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
					}
					c.Run(c, nil)
					break
				}
			}

			fmt.Println("\nSync complete!")
		},
	}.ToCobra()
}
