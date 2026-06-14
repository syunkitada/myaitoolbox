package profile

import (
	"fmt"
)

// Validate checks if the given profile is valid.
func Validate(p *Profile) error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	for serverName, srvConfig := range p.Servers {
		switch srvConfig.Transport {
		case "stdio":
			if srvConfig.Command == "" {
				return fmt.Errorf("server %s: 'command' is required for stdio transport", serverName)
			}
		case "streamable-http", "sse":
			if srvConfig.URL == "" {
				return fmt.Errorf("server %s: 'url' is required for %s transport", serverName, srvConfig.Transport)
			}
		default:
			return fmt.Errorf("server %s: unsupported transport '%s'", serverName, srvConfig.Transport)
		}
	}

	return nil
}
