package profile

import (
	"fmt"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

func (r *Resolver) Resolve(flagProfile, mcpProfile string) (*domain.Profile, error) {
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

func (r *Resolver) LoadConfig() (*domain.Config, error) {
	return LoadConfig()
}

func (r *Resolver) SaveConfig(cfg *domain.Config) error {
	return SaveConfig(cfg)
}

func (r *Resolver) ListProfiles() ([]string, error) {
	return ListProfiles()
}
