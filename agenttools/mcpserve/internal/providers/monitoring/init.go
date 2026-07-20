package monitoring

import "github.com/syunkitada/myaitoolbox/mcpserve/internal/infrastructure"

func init() {
	infrastructure.Register(New())
}
