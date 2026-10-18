package config

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// BrowserProfileDir returns the persistent Playwright profile directory for a
// project. The project name is hashed so a project name can never escape the
// application-owned directory or introduce path separators.
func BrowserProfileDir(project string) string {
	hash := sha256.Sum256([]byte(project))
	return filepath.Join(filepath.Dir(configPath()), "browser-profiles", hex.EncodeToString(hash[:]))
}
