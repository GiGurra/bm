package configcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/GiGurra/boa/pkg/boa"
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/gigurra/bm/pkg/config"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify settings",
	}

	cmd.AddCommand(listCmd(), addProfileCmd(), removeProfileCmd())
	return cmd
}

type ListParams struct{}

func listCmd() *cobra.Command {
	return boa.CmdT[ListParams]{
		Use:   "list",
		Short: "Show current settings",
		RunFunc: func(params *ListParams, cmd *cobra.Command, args []string) {
			cfg := config.Load()

			fmt.Printf("Config: %s\n\n", config.Path())

			if len(cfg.Profiles) == 0 {
				fmt.Println("Profiles: (none — using all Chrome profiles)")
				return
			}

			fmt.Println("Profiles:")
			for _, p := range cfg.Profiles {
				if p.Email != "" {
					fmt.Printf("  - email: %s\n", p.Email)
				} else if p.GaiaID != "" {
					fmt.Printf("  - gaia_id: %s\n", p.GaiaID)
				}
			}
		},
	}.ToCobra()
}

type AddProfileParams struct {
	Identifier string `pos:"true" help:"Chrome profile email or gaia_id to add"`
}

func addProfileCmd() *cobra.Command {
	cmd := boa.CmdT[AddProfileParams]{
		Use:   "add-profile",
		Short: "Add a Chrome profile to the config",
		RunFunc: func(params *AddProfileParams, cmd *cobra.Command, args []string) {
			id := params.Identifier

			// Match against discovered Chrome profiles to determine field type
			profiles, _ := chrome.DiscoverProfiles()
			var entry config.ProfileEntry
			matched := false
			for _, p := range profiles {
				if p.UserName == id {
					entry = config.ProfileEntry{Email: id}
					matched = true
					break
				}
				if p.GaiaID == id {
					entry = config.ProfileEntry{GaiaID: id}
					matched = true
					break
				}
			}
			if !matched {
				// Assume email if contains @, otherwise gaia_id
				if strings.Contains(id, "@") {
					entry = config.ProfileEntry{Email: id}
				} else {
					entry = config.ProfileEntry{GaiaID: id}
				}
				fmt.Fprintf(os.Stderr, "Warning: %q not found in current Chrome profiles, adding anyway\n", id)
			}

			cfg := config.Load()

			// Check for duplicates
			for _, existing := range cfg.Profiles {
				if existing.Email == entry.Email && entry.Email != "" {
					fmt.Printf("Profile %q already in config\n", entry.Email)
					return
				}
				if existing.GaiaID == entry.GaiaID && entry.GaiaID != "" {
					fmt.Printf("Profile %q already in config\n", entry.GaiaID)
					return
				}
			}

			cfg.Profiles = append(cfg.Profiles, entry)
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}

			label := entry.Email
			if label == "" {
				label = entry.GaiaID
			}
			fmt.Printf("Added profile %q (%d profile(s) in config)\n", label, len(cfg.Profiles))
		},
	}.ToCobra()
	cmd.ValidArgsFunction = addProfileValidArgs
	return cmd
}

type RemoveProfileParams struct {
	Identifier string `pos:"true" help:"Chrome profile email or gaia_id to remove"`
}

func removeProfileCmd() *cobra.Command {
	cmd := boa.CmdT[RemoveProfileParams]{
		Use:   "remove-profile",
		Short: "Remove a Chrome profile from the config",
		RunFunc: func(params *RemoveProfileParams, cmd *cobra.Command, args []string) {
			cfg := config.Load()
			id := params.Identifier

			var kept []config.ProfileEntry
			removed := false
			for _, p := range cfg.Profiles {
				if p.Email == id || p.GaiaID == id {
					removed = true
					continue
				}
				kept = append(kept, p)
			}

			if !removed {
				fmt.Fprintf(os.Stderr, "Profile %q not found in config\n", id)
				os.Exit(1)
			}

			cfg.Profiles = kept
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Removed profile %q (%d profile(s) remaining)\n", id, len(cfg.Profiles))
		},
	}.ToCobra()
	cmd.ValidArgsFunction = removeProfileValidArgs
	return cmd
}

// addProfileValidArgs suggests Chrome profiles not yet in the config.
func addProfileValidArgs(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	profiles, err := chrome.DiscoverProfiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg := config.Load()
	existing := make(map[string]bool)
	for _, p := range cfg.Profiles {
		if p.Email != "" {
			existing[p.Email] = true
		}
		if p.GaiaID != "" {
			existing[p.GaiaID] = true
		}
	}

	var alts []string
	for _, p := range profiles {
		if p.UserName != "" && !existing[p.UserName] {
			alts = append(alts, p.UserName)
		}
	}
	return alts, cobra.ShellCompDirectiveNoFileComp
}

// removeProfileValidArgs suggests profiles currently in the config.
func removeProfileValidArgs(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg := config.Load()
	var alts []string
	for _, p := range cfg.Profiles {
		if p.Email != "" {
			alts = append(alts, p.Email)
		} else if p.GaiaID != "" {
			alts = append(alts, p.GaiaID)
		}
	}
	return alts, cobra.ShellCompDirectiveNoFileComp
}
