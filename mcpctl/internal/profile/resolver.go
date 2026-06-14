package profile

import (
	"fmt"
)

// ResolveProfile determines which profile to load based on the resolution rules:
// 1. Explicitly provided (e.g., from --profile flag)
// 2. From MCP request (mcpProfile)
// 3. Default profile from config.yaml
func ResolveProfile(flagProfile, mcpProfile string) (*Profile, error) {
	var profileName string

	if flagProfile != "" {
		profileName = flagProfile
	} else if mcpProfile != "" {
		profileName = mcpProfile
	} else {
		cfg, err := LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load config to resolve default profile: %w", err)
		}
		if cfg.DefaultProfile == "" {
			return nil, fmt.Errorf("no profile specified and default_profile is not set in config")
		}
		profileName = cfg.DefaultProfile
	}

	return LoadProfile(profileName)
}
