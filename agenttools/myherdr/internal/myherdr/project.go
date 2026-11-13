package myherdr

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

type Project struct {
	Root string
	Name string
}

func ResolveProject(path string) (Project, error) {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return Project{}, fmt.Errorf("get current directory: %w", err)
		}
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return Project{}, fmt.Errorf("resolve project path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return Project{}, fmt.Errorf("inspect project path: %w", err)
	}
	if !info.IsDir() {
		return Project{}, errors.New("project path must be a directory")
	}

	root := absolute
	if git, err := exec.LookPath("git"); err == nil {
		command := exec.Command(git, "-C", absolute, "rev-parse", "--show-toplevel")
		output, commandErr := command.Output()
		if commandErr == nil {
			candidate := strings.TrimSpace(string(bytes.TrimSpace(output)))
			if candidate != "" {
				if resolved, absErr := filepath.Abs(candidate); absErr == nil {
					root = resolved
				}
			}
		}
	}

	name := filepath.Base(filepath.Clean(root))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "project"
	}
	return Project{Root: root, Name: name}, nil
}

func DefaultSessionName(projectName string) string {
	const maxSlugBytes = 64 - len("codex-")
	var result strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(projectName) {
		if r < unicode.MaxASCII && ((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			result.WriteRune(r)
			lastDash = false
			continue
		}
		if result.Len() > 0 && !lastDash {
			result.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(result.String(), "-")
	if slug == "" {
		slug = "project"
	}
	if len(slug) > maxSlugBytes {
		slug = strings.TrimRight(slug[:maxSlugBytes], "-")
	}
	return "codex-" + slug
}
