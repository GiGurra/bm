package main

import (
	"fmt"
	"os"

	"github.com/gigurra/bm/cmd/clear"
	"github.com/gigurra/bm/cmd/configcmd"
	"github.com/gigurra/bm/cmd/fetch"
	"github.com/gigurra/bm/cmd/importcmd"
	"github.com/gigurra/bm/cmd/index"
	"github.com/gigurra/bm/cmd/list"
	"github.com/gigurra/bm/cmd/search"
	"github.com/gigurra/bm/cmd/stats"
	"github.com/gigurra/bm/cmd/sync"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "bm",
		Short: "Searchable bookmark database with text and semantic search",
	}

	root.AddCommand(
		importcmd.Cmd(),
		fetch.Cmd(),
		index.Cmd(),
		search.Cmd(),
		list.Cmd(),
		sync.Cmd(),
		clear.Cmd(),
		stats.Cmd(),
		configcmd.Cmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
