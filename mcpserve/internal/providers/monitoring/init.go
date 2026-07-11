package monitoring

import "github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"

func init() {
	domain.Register(New())
}
