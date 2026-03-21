package clear

import (
	"fmt"
	"os"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/db"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear data from the database",
	}

	cmd.AddCommand(contentsCmd(), embeddingsCmd(), allCmd())
	return cmd
}

type ContentsParams struct{}

func contentsCmd() *cobra.Command {
	return boa.CmdT[ContentsParams]{
		Use:   "contents",
		Short: "Clear fetched page content (keeps bookmarks and embeddings)",
		RunFunc: func(params *ContentsParams, cmd *cobra.Command, args []string) {
			d, err := db.Open()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			res, err := d.Exec(`UPDATE bookmarks SET content_text='', fetched_at='', fetch_status=''`)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			n, _ := res.RowsAffected()
			fmt.Printf("Cleared content from %d bookmarks\n", n)
		},
	}.ToCobra()
}

type EmbeddingsParams struct{}

func embeddingsCmd() *cobra.Command {
	return boa.CmdT[EmbeddingsParams]{
		Use:   "embeddings",
		Short: "Clear all embeddings (keeps bookmarks and content)",
		RunFunc: func(params *EmbeddingsParams, cmd *cobra.Command, args []string) {
			d, err := db.Open()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			res, err := d.Exec(`DELETE FROM bookmark_embeddings`)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			n, _ := res.RowsAffected()
			fmt.Printf("Deleted %d embedding rows\n", n)
		},
	}.ToCobra()
}

type AllParams struct{}

func allCmd() *cobra.Command {
	return boa.CmdT[AllParams]{
		Use:   "all",
		Short: "Clear everything (bookmarks, content, and embeddings)",
		RunFunc: func(params *AllParams, cmd *cobra.Command, args []string) {
			d, err := db.Open()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if _, err := d.Exec(`DELETE FROM bookmark_embeddings`); err != nil {
				fmt.Fprintf(os.Stderr, "Error clearing embeddings: %v\n", err)
				os.Exit(1)
			}
			if _, err := d.Exec(`DELETE FROM bookmarks`); err != nil {
				fmt.Fprintf(os.Stderr, "Error clearing bookmarks: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Cleared all bookmarks, content, and embeddings")
		},
	}.ToCobra()
}
