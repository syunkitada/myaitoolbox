package markdown

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

const executeTimeout = 2 * time.Minute

type FileRepository struct {
	root string
}

func NewFileRepository(root string) *FileRepository {
	return &FileRepository{root: root}
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// safeAbsolutePath rejects paths whose existing components resolve outside the
// project root. Lexical validation alone is insufficient because a project can
// contain symlinks to arbitrary locations.
func (r *FileRepository) safeAbsolutePath(candidate string) (string, error) {
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		return "", err
	}
	probe := candidate
	for {
		resolved, resolveErr := filepath.EvalSymlinks(probe)
		if resolveErr == nil {
			if !pathWithin(root, resolved) {
				return "", fmt.Errorf("%w: %q", domain.ErrInvalidPath, candidate)
			}
			return candidate, nil
		}
		if !os.IsNotExist(resolveErr) {
			return "", resolveErr
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", fmt.Errorf("%w: %q", domain.ErrInvalidPath, candidate)
		}
		probe = parent
	}
}

func (r *FileRepository) safePath(path string) (string, error) {
	if err := validateFilePath(path); err != nil {
		return "", err
	}
	return r.safeAbsolutePath(filepath.Join(r.root, filepath.FromSlash(path)))
}

func (r *FileRepository) Tree(ctx context.Context, showHidden bool) ([]domain.FileEntry, error) {
	var entries []domain.FileEntry
	if _, err := os.Stat(r.root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	err := filepath.WalkDir(r.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == r.root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !showHidden && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(r.root, path)
		if relErr != nil {
			return relErr
		}
		kind := domain.FileKindFile
		if d.IsDir() {
			kind = domain.FileKindDir
		}
		status := ""
		if kind == domain.FileKindFile && d.Name() == "task.md" {
			status = markdownStatus(path)
		}
		entries = append(entries, domain.FileEntry{
			Path:       filepath.ToSlash(rel),
			Name:       d.Name(),
			Kind:       kind,
			Status:     status,
			Executable: execBit(d),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind == domain.FileKindDir
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

// Children lists only the direct children of parent ("" = project root),
// avoiding a full recursive walk of the tree.
func (r *FileRepository) Children(ctx context.Context, parent string, showHidden bool) ([]domain.FileEntry, error) {
	dir := r.root
	if parent != "" {
		var err error
		dir, err = r.safePath(parent)
		if err != nil {
			return nil, err
		}
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", domain.ErrInvalidPath, parent)
	}
	children, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.FileEntry, 0, len(children))
	for _, d := range children {
		if !showHidden && strings.HasPrefix(d.Name(), ".") {
			continue
		}
		rel := d.Name()
		if parent != "" {
			rel = parent + "/" + d.Name()
		}
		kind := domain.FileKindFile
		if d.IsDir() {
			kind = domain.FileKindDir
		}
		status := ""
		switch {
		case kind == domain.FileKindFile && d.Name() == "task.md" && d.Type()&os.ModeSymlink == 0:
			status = markdownStatus(filepath.Join(dir, d.Name()))
		case kind == domain.FileKindDir && parent == "tasks":
			taskPath := filepath.Join(dir, d.Name(), "task.md")
			if taskInfo, statErr := os.Lstat(taskPath); statErr == nil && taskInfo.Mode()&os.ModeSymlink == 0 {
				status = markdownStatus(taskPath)
			}
		}
		entries = append(entries, domain.FileEntry{
			Path:       filepath.ToSlash(rel),
			Name:       d.Name(),
			Kind:       kind,
			Status:     status,
			Executable: execBit(d),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind == domain.FileKindDir
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func (r *FileRepository) MarkdownTags(ctx context.Context) ([]string, error) {
	set := map[string]struct{}{}
	err := filepath.WalkDir(r.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == r.root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !isMarkdownPath(d.Name()) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmStr, _, ok := splitFrontMatter(string(content))
		if !ok {
			return nil
		}
		var fields struct {
			Tags []string `yaml:"tags"`
		}
		if err := yaml.Unmarshal([]byte(fmStr), &fields); err != nil {
			// Skip files whose front matter is not valid YAML so that one
			// malformed file does not break metadata collection for the whole
			// project. The invalid file is surfaced later when it is opened.
			return nil
		}
		for _, tag := range fields.Tags {
			if tag != "" {
				set[tag] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(set))
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}

func isMarkdownPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func execBit(d fs.DirEntry) bool {
	if d.IsDir() {
		return false
	}
	info, err := d.Info()
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}

func markdownStatus(path string) string {
	if !isMarkdownPath(path) {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return extractStatus(string(data))
}

func (r *FileRepository) Content(ctx context.Context, path string) (string, error) {
	file, err := r.safePath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, path)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *FileRepository) Raw(ctx context.Context, path string) ([]byte, error) {
	file, err := r.safePath(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, path)
	}
	return os.ReadFile(file)
}

func (r *FileRepository) Save(ctx context.Context, path string, content string) error {
	return r.SaveBytes(ctx, path, []byte(content))
}

func (r *FileRepository) SaveBytes(ctx context.Context, path string, content []byte) error {
	file, err := r.safePath(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		if info.IsDir() {
			return fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, path)
		}
		mode = info.Mode().Perm()
	}
	return fsutil.WriteFileAtomic(file, content, mode)
}

func (r *FileRepository) Create(ctx context.Context, path string) error {
	file, err := r.safePath(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, path)
	}
	return fsutil.WriteFileAtomic(file, nil, 0o644)
}

func (r *FileRepository) CreateDir(ctx context.Context, path string) error {
	dir, err := r.safePath(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, path)
	}
	return os.MkdirAll(dir, 0o755)
}

func (r *FileRepository) Move(ctx context.Context, oldPath string, newPath string) error {
	oldFile, err := r.safePath(oldPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, oldPath)
		}
		return err
	}
	target, err := r.safePath(newPath)
	if err != nil {
		return err
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		target = filepath.Join(target, filepath.Base(oldFile))
		if target, err = r.safeAbsolutePath(target); err != nil {
			return err
		}
	}
	if target == oldFile {
		return nil
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, newPath)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if isGitRepo(r.root) {
		if err := runGit(ctx, r.root, "mv", oldFile, target); err == nil {
			return nil
		}
	}
	return os.Rename(oldFile, target)
}

func (r *FileRepository) Copy(ctx context.Context, oldPath string, newPath string) error {
	oldFile, err := r.safePath(oldPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(oldFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, oldPath)
		}
		return err
	}
	target, err := r.safePath(newPath)
	if err != nil {
		return err
	}
	if targetInfo, err := os.Stat(target); err == nil && targetInfo.IsDir() {
		target = filepath.Join(target, filepath.Base(oldFile))
		if target, err = r.safeAbsolutePath(target); err != nil {
			return err
		}
	}
	if target == oldFile {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, newPath)
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, newPath)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(oldFile, target)
	}
	data, err := os.ReadFile(oldFile)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(target, data, info.Mode().Perm())
}

func copyDir(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink %q cannot be copied", domain.ErrInvalidPath, path)
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return fsutil.WriteFileAtomic(target, data, info.Mode().Perm())
	})
}

func (r *FileRepository) Delete(ctx context.Context, path string) error {
	file, err := r.safePath(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(file)
	}
	return os.Remove(file)
}

func (r *FileRepository) Execute(ctx context.Context, path string) (domain.FileExecResult, error) {
	file, err := r.safePath(path)
	if err != nil {
		return domain.FileExecResult{}, err
	}
	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.FileExecResult{}, fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return domain.FileExecResult{}, err
	}
	if info.IsDir() {
		return domain.FileExecResult{}, fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, path)
	}
	if info.Mode()&0o111 == 0 {
		return domain.FileExecResult{}, fmt.Errorf("%w: %s is not executable", domain.ErrInvalidPath, path)
	}
	ctx2, cancel := context.WithTimeout(ctx, executeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx2, file)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Dir = filepath.Dir(file)
	err = cmd.Run()
	res := domain.FileExecResult{Output: buf.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		} else if ctx2.Err() == context.DeadlineExceeded {
			res.TimedOut = true
			res.ExitCode = 124
			out := buf.String()
			if out != "" && !strings.HasSuffix(out, "\n") {
				out += "\n"
			}
			res.Output = out + "[timed out after " + executeTimeout.String() + "]"
		} else {
			return domain.FileExecResult{}, err
		}
	}
	return res, nil
}

func validateFilePath(path string) error {
	if path == "" || path == "." || path == ".." ||
		strings.HasPrefix(path, "/") || strings.Contains(path, "..") ||
		strings.ContainsAny(path, `\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, path)
	}
	return nil
}
