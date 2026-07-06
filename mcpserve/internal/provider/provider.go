package provider

// Provider defines the interface for an MCP Server implementation.
type Provider interface {
	Name() string
	Description() string
	NewServer() Server
}
