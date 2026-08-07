package markdown

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type KnowledgeRepository struct {
	root string
}

func NewKnowledgeRepository(root string) *KnowledgeRepository {
	return &KnowledgeRepository{root: root}
}

type knowledgeFields struct {
	Title   string    `yaml:"title"`
	Aliases []string  `yaml:"aliases"`
	Tags    []string  `yaml:"tags"`
	Type    string    `yaml:"type"`
	Created time.Time `yaml:"created"`
	LastMod time.Time `yaml:"lastmod"`
}

func (r *KnowledgeRepository) List(ctx context.Context) ([]domain.Knowledge, error) {
	return r.walk(ctx, filepath.Join(r.root, "knowledge"), "", false)
}

// ListScoped walks markdown notes under a scope directory and returns them with
// project-root-relative paths. An empty scope walks the whole project root,
// skipping hidden entries and the managed tasks/archives directories.
func (r *KnowledgeRepository) ListScoped(ctx context.Context, scope string) ([]domain.Knowledge, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	list, err := r.walk(ctx, r.root, "", scope == "")
	if err != nil {
		return nil, err
	}
	if scope == "" {
		return list, nil
	}
	var out []domain.Knowledge
	for _, k := range list {
		if k.Path == scope || strings.HasPrefix(k.Path, scope+"/") {
			out = append(out, k)
		}
	}
	return out, nil
}

func (r *KnowledgeRepository) walk(ctx context.Context, dir string, base string, skipManaged bool) ([]domain.Knowledge, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var knowledge []domain.Knowledge
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() && skipManaged && (name == "tasks" || name == "archives") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(base, name))
		if e.IsDir() {
			sub, err := r.walk(ctx, filepath.Join(dir, name), rel, skipManaged)
			if err != nil {
				return nil, err
			}
			knowledge = append(knowledge, sub...)
			continue
		}
		if strings.HasSuffix(name, ".md") {
			path := strings.TrimSuffix(rel, ".md")
			k, err := r.read(filepath.Join(dir, name), path)
			if err != nil {
				return nil, err
			}
			knowledge = append(knowledge, *k)
		}
	}
	return knowledge, nil
}

func validateScope(scope string) error {
	if scope == "" {
		return nil
	}
	if scope == "." || scope == ".." || strings.HasPrefix(scope, "/") ||
		strings.Contains(scope, "..") || strings.ContainsAny(scope, `\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, scope)
	}
	return nil
}

func (r *KnowledgeRepository) read(path string, relPath string) (*domain.Knowledge, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fmStr, body, ok := splitFrontMatter(string(content))
	var f knowledgeFields
	if ok {
		if err := yaml.Unmarshal([]byte(fmStr), &f); err != nil {
			return nil, err
		}
	}
	title := f.Title
	if title == "" {
		title = extractTitle(body)
	}
	return &domain.Knowledge{
		Path:      relPath,
		Title:     title,
		Aliases:   f.Aliases,
		Tags:      f.Tags,
		Type:      f.Type,
		Created:   f.Created,
		LastMod:   f.LastMod,
		WikiLinks: extractWikiLinks(body),
		Body:      strings.TrimPrefix(body, "\n"),
	}, nil
}

func (r *KnowledgeRepository) Find(ctx context.Context, path string) (*domain.Knowledge, error) {
	file, err := r.resolve(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(file); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return nil, err
	}
	return r.read(file, path)
}

func (r *KnowledgeRepository) RawContent(ctx context.Context, path string) (string, error) {
	file, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return "", err
	}
	return string(content), nil
}

func (r *KnowledgeRepository) Create(ctx context.Context, path string, content string) error {
	if err := validateKnowledgePath(path); err != nil {
		return err
	}
	file := filepath.Join(r.root, "knowledge", path+".md")
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, path)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(content), 0o644)
}

func (r *KnowledgeRepository) SaveContent(ctx context.Context, path string, content string) error {
	if err := validateKnowledgePath(path); err != nil {
		return err
	}
	file := filepath.Join(r.root, "knowledge", path+".md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(content), 0o644)
}

func (r *KnowledgeRepository) Move(ctx context.Context, oldPath string, newPath string) error {
	if err := validateKnowledgePath(oldPath); err != nil {
		return err
	}
	if err := validateKnowledgePath(newPath); err != nil {
		return err
	}
	oldFile := filepath.Join(r.root, "knowledge", oldPath+".md")
	newFile := filepath.Join(r.root, "knowledge", newPath+".md")
	if oldFile == newFile {
		return nil
	}
	if _, err := os.Stat(newFile); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, newPath)
	}
	if err := os.MkdirAll(filepath.Dir(newFile), 0o755); err != nil {
		return err
	}
	if isGitRepo(r.root) {
		return runGit(ctx, r.root, "mv", oldFile, newFile)
	}
	return os.Rename(oldFile, newFile)
}

func (r *KnowledgeRepository) Rename(ctx context.Context, oldPath string, newName string) error {
	if err := validateKnowledgePath(oldPath); err != nil {
		return err
	}
	if newName == "" || strings.ContainsAny(newName, `/\`) || strings.Contains(newName, "..") {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, newName)
	}
	dir := filepath.Dir(oldPath)
	newPath := filepath.Join(dir, newName)
	return r.Move(ctx, oldPath, newPath)
}

func (r *KnowledgeRepository) resolve(path string) (string, error) {
	if err := validateKnowledgePath(path); err != nil {
		return "", err
	}
	return filepath.Join(r.root, "knowledge", path+".md"), nil
}

func validateKnowledgePath(path string) error {
	if path == "" || path == "." || path == ".." {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, path)
	}
	if strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.ContainsAny(path, `\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, path)
	}
	return nil
}

func extractTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

var wikiLinkPattern = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]*)?\]\]`)

func extractWikiLinks(body string) []string {
	var links []string
	for _, m := range wikiLinkPattern.FindAllStringSubmatch(body, -1) {
		links = append(links, strings.TrimSpace(m[1]))
	}
	return links
}
