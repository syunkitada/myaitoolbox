package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultProfile string `yaml:"default_profile"`
	Cache          struct {
		Enabled bool   `yaml:"enabled"`
		TTL     string `yaml:"ttl"`
	} `yaml:"cache"`
	Output struct {
		Format string `yaml:"format"`
	} `yaml:"output"`
}

type Profile struct {
	Name    string                  `yaml:"name"`
	Servers map[string]ServerConfig `yaml:"servers"`
}

type ServerConfig struct {
	Transport string `yaml:"transport"`
	Command   string `yaml:"command,omitempty"`
	URL       string `yaml:"url,omitempty"`
}

// GetConfigDir returns the path to the mcpctl config directory.
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir: %w", err)
	}
	return filepath.Join(home, ".config", "mcpctl"), nil
}

// LoadConfig loads the main config.yaml file.
func LoadConfig() (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				DefaultProfile: "default",
				Output: struct {
					Format string `yaml:"format"`
				}{Format: "table"},
			}, nil // Return default if config doesn't exist
		}
		return nil, fmt.Errorf("failed to read config.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the config.yaml file.
func SaveConfig(cfg *Config) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}

	return nil
}

// LoadProfile loads a profile by name from profiles/<name>.yaml.
func LoadProfile(name string) (*Profile, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	profilePath := filepath.Join(configDir, "profiles", fmt.Sprintf("%s.yaml", name))
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile %s: %w", name, err)
	}

	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile %s: %w", name, err)
	}
	
	if profile.Name == "" {
		profile.Name = name
	}

	return &profile, nil
}

// ListProfiles lists all available profiles in the profiles/ directory.
func ListProfiles() ([]string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	profilesDir := filepath.Join(configDir, "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read profiles dir: %w", err)
	}

	var profiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			name := entry.Name()[:len(entry.Name())-5]
			profiles = append(profiles, name)
		}
	}

	return profiles, nil
}
