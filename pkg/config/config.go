package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gigurra/bm/pkg/chrome"
)

// ProfileEntry identifies a Chrome profile by email or Google account ID.
type ProfileEntry struct {
	Email  string `json:"email,omitempty"`
	GaiaID string `json:"gaia_id,omitempty"`
}

// Config holds bm settings loaded from ~/.bm/settings.json.
type Config struct {
	Profiles []ProfileEntry `json:"profiles,omitempty"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bm")
}

func Path() string {
	return filepath.Join(Dir(), "settings.json")
}

// Load reads the config file. Returns zero Config if the file doesn't exist or is invalid.
func Load() Config {
	data, err := os.ReadFile(Path())
	if err != nil {
		return Config{}
	}
	var cfg Config
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// Save writes the config to ~/.bm/settings.json.
func Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(Path(), data, 0644)
}

// MatchesProfile returns true if the Chrome profile matches this config entry.
func (e ProfileEntry) MatchesProfile(p chrome.Profile) bool {
	if e.Email != "" && p.UserName == e.Email {
		return true
	}
	if e.GaiaID != "" && p.GaiaID == e.GaiaID {
		return true
	}
	return false
}

func matchesProfileFilter(p chrome.Profile, filter string) bool {
	return p.DirName == filter || p.UserName == filter || p.Name == filter || p.GaiaID == filter
}

// ResolveProfiles determines which Chrome profiles to use.
// Priority: CLI/env > config file > all.
// Returns (nil, nil) to mean "use all profiles".
func ResolveProfiles(cliProfile string) ([]chrome.Profile, error) {
	if cliProfile == "all" {
		return nil, nil
	}

	allProfiles, err := chrome.DiscoverProfiles()
	if err != nil {
		return nil, err
	}

	if cliProfile != "" {
		for _, p := range allProfiles {
			if matchesProfileFilter(p, cliProfile) {
				return []chrome.Profile{p}, nil
			}
		}
		return nil, fmt.Errorf("profile %q not found. Available: %s", cliProfile, profileNames(allProfiles))
	}

	// Check config file
	cfg := Load()
	if len(cfg.Profiles) == 0 {
		return nil, nil
	}

	var matched []chrome.Profile
	for _, entry := range cfg.Profiles {
		for _, p := range allProfiles {
			if entry.MatchesProfile(p) {
				matched = append(matched, p)
				break
			}
		}
	}

	if len(matched) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: no Chrome profiles matched config, using all profiles\n")
		return nil, nil
	}
	return matched, nil
}

// ResolveSourceIDs returns source IDs for DB queries.
// Returns nil to mean "no filtering" (all sources).
func ResolveSourceIDs(cliProfile string) ([]string, error) {
	profiles, err := ResolveProfiles(cliProfile)
	if err != nil {
		return nil, err
	}
	if profiles == nil {
		return nil, nil
	}
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.SourceID()
	}
	return ids, nil
}

func profileNames(profiles []chrome.Profile) string {
	if len(profiles) == 0 {
		return "(none)"
	}
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.DisplayName()
	}
	return strings.Join(names, ", ")
}
