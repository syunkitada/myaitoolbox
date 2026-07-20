package application

import (
	"fmt"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func ListProfiles(resolver domain.ProfileResolver) (string, error) {
	cfg, err := resolver.LoadConfig()
	if err != nil {
		return "", err
	}

	profs, err := resolver.ListProfiles()
	if err != nil {
		return "", err
	}

	if len(profs) == 0 {
		return "No profiles found in ~/.config/mcpctl/profiles/", nil
	}

	var out string
	for _, p := range profs {
		if p == cfg.DefaultProfile {
			out += fmt.Sprintf("* %s\n", p)
		} else {
			out += fmt.Sprintf("  %s\n", p)
		}
	}

	return out, nil
}

func GetCurrentProfile(resolver domain.ProfileResolver, flagProfile string) (string, error) {
	p, err := resolver.Resolve(flagProfile, "")
	if err != nil {
		return "", err
	}
	return p.Name, nil
}

func UseProfile(resolver domain.ProfileResolver, newName string) (string, error) {
	cfg, err := resolver.LoadConfig()
	if err != nil {
		return "", err
	}

	oldProfile := cfg.DefaultProfile
	cfg.DefaultProfile = newName

	if err := resolver.SaveConfig(cfg); err != nil {
		return "", err
	}

	return fmt.Sprintf("default profile changed:\n  %s -> %s", oldProfile, newName), nil
}
