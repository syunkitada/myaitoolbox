package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"raw", false},
		{"tsv", false},
		{"table", false},
		{"json", true},
		{"yaml", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			err := ValidateOutputFormat(tt.format)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseCallArgs(t *testing.T) {
	t.Run("simple tool path", func(t *testing.T) {
		args := []string{"server/tool"}
		toolPath, params, outputFormat, err := ParseCallArgs(args, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "server/tool", toolPath)
		assert.Equal(t, "tsv", outputFormat)
		assert.Empty(t, params)
	})

	t.Run("with output flag", func(t *testing.T) {
		args := []string{"server/tool", "-o", "table"}
		toolPath, _, outputFormat, err := ParseCallArgs(args, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "server/tool", toolPath)
		assert.Equal(t, "table", outputFormat)
	})

	t.Run("with params JSON", func(t *testing.T) {
		args := []string{"server/tool", "--params", `{"key":"value"}`}
		toolPath, params, _, err := ParseCallArgs(args, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "server/tool", toolPath)
		assert.Equal(t, "value", params["key"])
	})

	t.Run("with inline param", func(t *testing.T) {
		args := []string{"server/tool", "--name", "test"}
		toolPath, params, _, err := ParseCallArgs(args, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "server/tool", toolPath)
		assert.Equal(t, "test", params["name"])
	})

	t.Run("profile flag is skipped", func(t *testing.T) {
		args := []string{"server/tool", "--profile", "myprofile"}
		toolPath, params, _, err := ParseCallArgs(args, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "server/tool", toolPath)
		assert.Nil(t, params["profile"])
	})
}

func TestFormatParamList(t *testing.T) {
	t.Run("nil schema", func(t *testing.T) {
		entry := &domain.ToolEntry{
			ServerName: "server",
			Tool: &mcp.Tool{
				Name:        "tool",
				Description: "test",
				InputSchema: nil,
			},
		}
		result := FormatParamList(entry)
		assert.Equal(t, "(no parameters)", result)
	})

	t.Run("empty properties", func(t *testing.T) {
		entry := &domain.ToolEntry{
			ServerName: "server",
			Tool: &mcp.Tool{
				Name:        "tool",
				Description: "test",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		}
		result := FormatParamList(entry)
		assert.Equal(t, "(no parameters)", result)
	})

	t.Run("with parameters", func(t *testing.T) {
		entry := &domain.ToolEntry{
			ServerName: "server",
			Tool: &mcp.Tool{
				Name:        "tool",
				Description: "test",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"count": map[string]interface{}{
							"type": "integer",
						},
					},
					"required": []interface{}{"name"},
				},
			},
		}
		result := FormatParamList(entry)
		assert.Contains(t, result, "name: string")
		assert.Contains(t, result, "(required)")
		assert.Contains(t, result, "count: integer")
	})
}

type callTestDiscovery struct {
	entry domain.ToolEntry
}

func (d callTestDiscovery) ListTools(context.Context, *domain.Profile, string) ([]domain.ToolEntry, error) {
	return []domain.ToolEntry{d.entry}, nil
}

func (d callTestDiscovery) GetToolInfo(context.Context, *domain.Profile, string, string) (*domain.ToolEntry, error) {
	return &d.entry, nil
}

func (d callTestDiscovery) SearchTools(context.Context, *domain.Profile, string) ([]domain.ToolEntry, error) {
	return []domain.ToolEntry{d.entry}, nil
}

func TestParseCallArgsConvertsSchemaTypes(t *testing.T) {
	discovery := callTestDiscovery{entry: domain.ToolEntry{
		ServerName: "server",
		Tool: &mcp.Tool{
			Name: "tool",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"count":   map[string]interface{}{"type": "integer"},
					"ratio":   map[string]interface{}{"type": "number"},
					"enabled": map[string]interface{}{"type": "boolean"},
					"tags":    map[string]interface{}{"type": "array"},
				},
			},
		},
	}}
	profile := &domain.Profile{Name: "test"}

	toolPath, params, output, profileName, err := ParseCallArgsContext(context.Background(), []string{
		"server/tool", "--count", "3", "--ratio", "1.5", "--enabled", "false", "--tags", "a,b", "--profile", "prod",
	}, discovery, profile, "table")
	require.NoError(t, err)
	assert.Equal(t, "server/tool", toolPath)
	assert.Equal(t, "table", output)
	assert.Equal(t, "prod", profileName)
	assert.Equal(t, json.Number("3"), params["count"])
	assert.Equal(t, json.Number("1.5"), params["ratio"])
	assert.Equal(t, false, params["enabled"])
	assert.Equal(t, []interface{}{"a", "b"}, params["tags"])
}

func TestParseCallArgsRejectsMissingFlagValue(t *testing.T) {
	_, _, _, _, err := ParseCallArgsContext(context.Background(), []string{"server/tool", "--output"}, nil, nil, "table")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a value")
}

func TestParseCallTarget(t *testing.T) {
	target, profile, err := ParseCallTarget([]string{"--profile", "prod", "server/tool", "-h"})
	require.NoError(t, err)
	assert.Equal(t, "server/tool", target)
	assert.Equal(t, "prod", profile)
}
