package domain

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// GenerateTaskID creates a unique, sortable task ID.
// Format: YYYYMMDD-HHMMSS-<source>-<short-hash>
func GenerateTaskID(source string, eventData []byte) string {
	now := time.Now()
	hash := sha256.Sum256(eventData)
	shortHash := fmt.Sprintf("%x", hash[:3])
	return fmt.Sprintf("%s-%s-%s-%s",
		now.Format("20060102"),
		now.Format("150405"),
		source,
		shortHash,
	)
}
