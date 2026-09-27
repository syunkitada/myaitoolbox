package myntfy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	RandomSuffixBytes  = 16
	RandomSuffixLength = RandomSuffixBytes * 2
	TopicSeparator     = "-"
	MaxTopicLength     = 64
	MaxPrefixLength    = MaxTopicLength - len(TopicSeparator) - RandomSuffixLength
)

var ErrTopicNotConfigured = errors.New("topic is not configured")

// TopicStore persists the ntfy topic used by myntfy.
type TopicStore struct {
	path   string
	random io.Reader
}

func NewTopicStore(path string) *TopicStore {
	return &TopicStore{path: path, random: rand.Reader}
}

func DefaultTopicPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("MYNTFY_CONFIG")); path != "" {
		return path, nil
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "myntfy", "topic"), nil
}

func (s *TopicStore) Read() (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: run `myntfy topic regenerate --prefix <prefix>`", ErrTopicNotConfigured)
		}
		return "", fmt.Errorf("read topic: %w", err)
	}

	topic := strings.TrimSpace(string(data))
	if err := ValidateTopic(topic); err != nil {
		return "", fmt.Errorf("read topic: %w", err)
	}
	return topic, nil
}

func (s *TopicStore) Regenerate(prefix string) (string, error) {
	if err := ValidatePrefix(prefix); err != nil {
		return "", fmt.Errorf("generate topic: %w", err)
	}
	if s.random == nil {
		s.random = rand.Reader
	}

	buf := make([]byte, RandomSuffixBytes)
	if _, err := io.ReadFull(s.random, buf); err != nil {
		return "", fmt.Errorf("generate topic: %w", err)
	}
	topic := prefix + TopicSeparator + hex.EncodeToString(buf)
	if err := ValidateTopic(topic); err != nil {
		return "", fmt.Errorf("generate topic: %w", err)
	}
	if err := s.save(topic); err != nil {
		return "", err
	}
	return topic, nil
}

func ValidatePrefix(prefix string) error {
	if prefix == "" {
		return errors.New("topic prefix is empty")
	}
	if len(prefix) > MaxPrefixLength {
		return fmt.Errorf("topic prefix is too long: maximum length is %d", MaxPrefixLength)
	}
	for _, r := range prefix {
		if isTopicCharacter(r) {
			continue
		}
		return fmt.Errorf("topic prefix contains unsupported character %q", r)
	}
	return nil
}

func ReplaceInvalidPrefixChars(prefix string) (string, bool) {
	var replaced strings.Builder
	changed := false
	for _, r := range prefix {
		if isTopicCharacter(r) {
			replaced.WriteRune(r)
			continue
		}
		replaced.WriteRune('-')
		changed = true
	}
	if !changed {
		return prefix, false
	}
	return replaced.String(), true
}

func ValidateTopic(topic string) error {
	if topic == "" {
		return errors.New("topic is empty")
	}
	if len(topic) > MaxTopicLength {
		return fmt.Errorf("topic is too long: maximum length is %d", MaxTopicLength)
	}
	for _, r := range topic {
		if isTopicCharacter(r) {
			continue
		}
		return fmt.Errorf("topic contains unsupported character %q", r)
	}
	return nil
}

func isTopicCharacter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '-'
}

func (s *TopicStore) save(topic string) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create topic directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".topic-*")
	if err != nil {
		return fmt.Errorf("create temporary topic file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set topic file permissions: %w", err)
	}
	if _, err := io.WriteString(tmp, topic+"\n"); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write topic: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync topic: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary topic file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace topic file: %w", err)
	}
	return nil
}
