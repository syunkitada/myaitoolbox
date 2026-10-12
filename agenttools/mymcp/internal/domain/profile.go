package domain

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
	Transport string   `yaml:"transport"`
	Command   string   `yaml:"command,omitempty"`
	Args      []string `yaml:"args,omitempty"`
	URL       string   `yaml:"url,omitempty"`
	Env       []string `yaml:"env,omitempty"`
}
