package myntfy

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestTopicStoreRegenerateAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "topic")
	store := NewTopicStore(path)
	store.random = bytes.NewReader(bytes.Repeat([]byte{0xab}, RandomSuffixBytes))

	topic, err := store.Regenerate("server01")
	if err != nil {
		t.Fatalf("Regenerate() error = %v", err)
	}
	if want := "server01-abababababababababababababababab"; topic != want {
		t.Fatalf("Regenerate() = %q, want %q", topic, want)
	}

	got, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != topic {
		t.Fatalf("Read() = %q, want %q", got, topic)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("topic file mode = %o, want 600", got)
	}
}

func TestTopicStoreRegenerateUsesRandomTopic(t *testing.T) {
	store := NewTopicStore(filepath.Join(t.TempDir(), "topic"))
	first, err := store.Regenerate("server-a")
	if err != nil {
		t.Fatalf("first Regenerate() error = %v", err)
	}
	second, err := store.Regenerate("server-a")
	if err != nil {
		t.Fatalf("second Regenerate() error = %v", err)
	}
	if first == second {
		t.Fatalf("two generated topics are identical: %q", first)
	}
	if ok, err := regexp.MatchString(`^server-a-[A-Fa-f0-9]{32}$`, second); err != nil || !ok {
		t.Fatalf("generated topic has unexpected format: %q", second)
	}
}

func TestValidatePrefix(t *testing.T) {
	for _, prefix := range []string{"server01", "server-a", "server_a"} {
		if err := ValidatePrefix(prefix); err != nil {
			t.Errorf("ValidatePrefix(%q) error = %v", prefix, err)
		}
	}
	for _, prefix := range []string{"", "server/a", "server a", string(bytes.Repeat([]byte{'a'}, MaxPrefixLength+1))} {
		if err := ValidatePrefix(prefix); err == nil {
			t.Errorf("ValidatePrefix(%q) error = nil, want error", prefix)
		}
	}
}

func TestReplaceInvalidPrefixChars(t *testing.T) {
	for _, test := range []struct {
		input   string
		want    string
		changed bool
	}{
		{input: "server01", want: "server01", changed: false},
		{input: "server.example", want: "server-example", changed: true},
		{input: "server/production", want: "server-production", changed: true},
	} {
		got, changed := ReplaceInvalidPrefixChars(test.input)
		if got != test.want || changed != test.changed {
			t.Errorf("ReplaceInvalidPrefixChars(%q) = %q, %t; want %q, %t", test.input, got, changed, test.want, test.changed)
		}
	}
}

func TestTopicStoreReadMissing(t *testing.T) {
	store := NewTopicStore(filepath.Join(t.TempDir(), "topic"))
	_, err := store.Read()
	if !errors.Is(err, ErrTopicNotConfigured) {
		t.Fatalf("Read() error = %v, want ErrTopicNotConfigured", err)
	}
}

func TestValidateTopic(t *testing.T) {
	valid := "abcXYZ012_-"
	if err := ValidateTopic(valid); err != nil {
		t.Fatalf("ValidateTopic(%q) error = %v", valid, err)
	}
	for _, topic := range []string{"", "a/b", "a b", string(bytes.Repeat([]byte{'a'}, MaxTopicLength+1))} {
		if err := ValidateTopic(topic); err == nil {
			t.Errorf("ValidateTopic(%q) error = nil, want error", topic)
		}
	}
}
