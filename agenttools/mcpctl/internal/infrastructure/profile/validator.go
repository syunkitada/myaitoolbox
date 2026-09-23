package profile

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func Validate(p *domain.Profile) error {
	if p == nil {
		return fmt.Errorf("profile is required")
	}
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	for serverName, srvConfig := range p.Servers {
		if strings.TrimSpace(serverName) == "" {
			return fmt.Errorf("server name is required")
		}
		switch srvConfig.Transport {
		case "stdio":
			if strings.TrimSpace(srvConfig.Command) == "" {
				return fmt.Errorf("server %s: 'command' is required for stdio transport", serverName)
			}
		case "streamable-http", "sse":
			if strings.TrimSpace(srvConfig.URL) == "" {
				return fmt.Errorf("server %s: 'url' is required for %s transport", serverName, srvConfig.Transport)
			}
			parsedURL, err := url.Parse(srvConfig.URL)
			if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
				return fmt.Errorf("server %s: invalid URL %q", serverName, srvConfig.URL)
			}
		default:
			return fmt.Errorf("server %s: unsupported transport '%s'", serverName, srvConfig.Transport)
		}

		for _, env := range srvConfig.Env {
			if idx := strings.IndexByte(env, '='); idx <= 0 {
				return fmt.Errorf("server %s: invalid environment entry %q; expected KEY=VALUE", serverName, env)
			}
		}
	}

	return nil
}
