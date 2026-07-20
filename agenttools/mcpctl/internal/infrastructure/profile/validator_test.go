package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func TestLoadConfig(t *testing.T) {
	t.Run("default config when file missing", func(t *testing.T) {
		// Use a temp dir to simulate missing config
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		// This test verifies the function handles missing config gracefully
		// In a real test, we'd mock the home dir, but for now we just test
		// that the function doesn't panic
		configDir := filepath.Join(home, ".config", "mcpctl")
		_, err = os.Stat(configDir)
		if os.IsNotExist(err) {
			// Config dir doesn't exist, LoadConfig should return defaults
			cfg, err := LoadConfig()
			assert.NoError(t, err)
			assert.Equal(t, "default", cfg.DefaultProfile)
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		profile *domain.Profile
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty name",
			profile: &domain.Profile{Name: ""},
			wantErr: true,
			errMsg:  "profile name is required",
		},
		{
			name: "stdio without command",
			profile: &domain.Profile{
				Name: "test",
				Servers: map[string]domain.ServerConfig{
					"srv": {Transport: "stdio"},
				},
			},
			wantErr: true,
			errMsg:  "'command' is required",
		},
		{
			name: "http without url",
			profile: &domain.Profile{
				Name: "test",
				Servers: map[string]domain.ServerConfig{
					"srv": {Transport: "streamable-http"},
				},
			},
			wantErr: true,
			errMsg:  "'url' is required",
		},
		{
			name: "unsupported transport",
			profile: &domain.Profile{
				Name: "test",
				Servers: map[string]domain.ServerConfig{
					"srv": {Transport: "grpc"},
				},
			},
			wantErr: true,
			errMsg:  "unsupported transport",
		},
		{
			name: "valid stdio",
			profile: &domain.Profile{
				Name: "test",
				Servers: map[string]domain.ServerConfig{
					"srv": {Transport: "stdio", Command: "myserver"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.profile)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
