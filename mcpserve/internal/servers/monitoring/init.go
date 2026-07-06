package monitoring

import "github.com/syunkitada/myaitoolbox/mcpserve/internal/registry"

func init() {
	registry.Register(New())
}
