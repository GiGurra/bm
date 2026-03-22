package common

import (
	"github.com/gigurra/bm/pkg/chrome"
	"github.com/spf13/cobra"
)

// ProfileAlternatives returns completion alternatives for --profile: emails + "all".
func ProfileAlternatives(_ *cobra.Command, _ []string, _ string) []string {
	profiles, err := chrome.DiscoverProfiles()
	if err != nil {
		return nil
	}
	alts := []string{"all"}
	for _, p := range profiles {
		if p.UserName != "" {
			alts = append(alts, p.UserName)
		}
	}
	return alts
}
