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
	return r.walk(ctx, filepath.Join(r.root, "knowledge"), "")
}

// ListScoped walks markdown notes under a scope directory and returns them with
// project-root-relative paths. An empty scope walks the whole project root,
// skipping hidden entries. Task files (task.md under tasks/ or archives/tasks/)
// are included and marked with Type "task".
func (r *KnowledgeRepository) ListScoped(ctx context.Context, scope string) ([]domain.Knowledge, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	list, err := r.walk(ctx, r.root, "")
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

func (r *KnowledgeRepository) walk(ctx context.Context, dir string, base string) ([]domain.Knowledge, error) {
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
		rel := filepath.ToSlash(filepath.Join(base, name))
		if e.IsDir() {
			sub, err := r.walk(ctx, filepath.Join(dir, name), rel)
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
			if isTaskFilePath(path) {
				k.Type = "task"
			}
			knowledge = append(knowledge, *k)
		}
	}
	return knowledge, nil
}

func isTaskFilePath(path string) bool {
	return strings.HasPrefix(path, "tasks/") || strings.HasPrefix(path, "archives/tasks/")
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
	mdLinks := extractMarkdownLinks(body)
	resolved := make([]string, 0, len(mdLinks))
	for _, l := range mdLinks {
		if r := resolveRelativeLink(l, filepath.Dir(relPath)); r != "" {
			resolved = append(resolved, r)
		}
	}
	return &domain.Knowledge{
		Path:          relPath,
		Title:         title,
		Aliases:       f.Aliases,
		Tags:          f.Tags,
		Type:          f.Type,
		Created:       f.Created,
		LastMod:       f.LastMod,
		WikiLinks:     extractWikiLinks(body),
		MarkdownLinks: resolved,
		Body:          strings.TrimPrefix(body, "\n"),
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

var markdownLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

func extractMarkdownLinks(body string) []string {
	var links []string
	for _, m := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		target := strings.TrimSpace(m[2])
		if !isMarkdownNoteTarget(target) {
			continue
		}
		links = append(links, strings.TrimPrefix(target, "./"))
	}
	return links
}

func isMarkdownNoteTarget(target string) bool {
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") {
		return false
	}
	if strings.ContainsAny(target, `\`) {
		return false
	}
	if scheme, _, ok := strings.Cut(target, ":"); ok && !strings.ContainsAny(scheme, "/.") {
		return false
	}
	clean := target
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	return strings.HasSuffix(strings.ToLower(clean), ".md") || strings.HasSuffix(clean, "/")
}

// resolveRelativeLink resolves a markdown link target relative to the
// directory of the note that contains it, returning a project-root-relative
// path. A trailing ".md" or "/" is stripped from the target.
func resolveRelativeLink(target string, dir string) string {
	clean := target
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	clean = strings.TrimSuffix(clean, ".md")
	clean = strings.TrimSuffix(clean, "/")
	parts := []string{}
	if dir != "" && dir != "." {
		parts = strings.Split(filepath.ToSlash(dir), "/")
	}
	for _, p := range strings.Split(clean, "/") {
		switch p {
		case "", ".":
			continue
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "/")
}
