package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseToolName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantServer string
		wantTool   string
		wantErr    bool
	}{
		{
			name:       "valid tool name",
			input:      "server1/tool1",
			wantServer: "server1",
			wantTool:   "tool1",
			wantErr:    false,
		},
		{
			name:       "valid with slash in tool name",
			input:      "server/sub/tool",
			wantServer: "server",
			wantTool:   "sub/tool",
			wantErr:    false,
		},
		{
			name:    "missing slash",
			input:   "server1tool1",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, tool, err := ParseToolName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantServer, server)
			assert.Equal(t, tt.wantTool, tool)
		})
	}
}
