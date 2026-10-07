package markdown

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

const executeTimeout = 2 * time.Minute

const maxSearchFileBytes = 5 << 20
const maxContentFileBytes = 32 << 20
const maxRawFileBytes = 128 << 20

var searchableTextExtensions = map[string]struct{}{
	".adoc": {}, ".bash": {}, ".c": {}, ".cc": {}, ".cfg": {}, ".conf": {}, ".cpp": {},
	".css": {}, ".csv": {}, ".fish": {}, ".go": {}, ".h": {}, ".hh": {}, ".hpp": {},
	".htm": {}, ".html": {}, ".ini": {}, ".java": {}, ".js": {}, ".json": {},
	".jsx": {}, ".kts": {}, ".less": {}, ".lua": {}, ".markdown": {}, ".md": {},
	".mjs": {}, ".org": {}, ".php": {}, ".pl": {}, ".ps1": {}, ".py": {},
	".rb": {}, ".rs": {}, ".rst": {}, ".scss": {}, ".sh": {}, ".sql": {},
	".svg": {}, ".swift": {}, ".text": {}, ".toml": {}, ".ts": {},
	".tsx": {}, ".txt": {}, ".xml": {}, ".yaml": {}, ".yml": {},
}

var searchableTextNames = map[string]struct{}{
	".dockerignore": {}, ".editorconfig": {}, ".env": {}, ".gitattributes": {}, ".gitignore": {},
	".npmrc": {}, ".prettierrc": {}, ".yarnrc": {}, "agentrules": {}, "agents.md": {},
	"dockerfile": {}, "gitignore": {}, "makefile": {}, "readme": {}, "license": {},
}

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
		if kind == domain.FileKindFile && isMarkdownPath(d.Name()) {
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

func (r *FileRepository) Search(ctx context.Context, query string, showHidden bool) ([]domain.FileSearchResult, error) {
	results, _, err := r.search(ctx, query, showHidden, 0)
	return results, err
}

// SearchLimited keeps the complete match count while retaining only the
// first limit results. The application layer exposes at most this many hits,
// so avoiding a second, unbounded result slice matters for large projects.
func (r *FileRepository) SearchLimited(ctx context.Context, query string, showHidden bool, limit int) ([]domain.FileSearchResult, int, error) {
	return r.search(ctx, query, showHidden, limit)
}

func (r *FileRepository) search(ctx context.Context, query string, showHidden bool, limit int) ([]domain.FileSearchResult, int, error) {
	if strings.TrimSpace(query) == "" {
		return nil, 0, fmt.Errorf("%w: search query must not be empty", domain.ErrInvalidArgument)
	}
	if _, err := os.Stat(r.root); err != nil {
		if os.IsNotExist(err) {
			return []domain.FileSearchResult{}, 0, nil
		}
		return nil, 0, err
	}

	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return nil, 0, fmt.Errorf("file search requires rg on PATH: %w", err)
	}

	args := []string{
		"--json",
		"--fixed-strings",
		"--ignore-case",
		"--glob-case-insensitive",
		"--no-ignore",
		fmt.Sprintf("--max-filesize=%d", maxSearchFileBytes),
	}
	if showHidden {
		args = append(args, "--hidden")
	}
	for _, pattern := range searchableTextGlobs() {
		args = append(args, "--glob", pattern)
	}
	args = append(args, "--", query, ".")

	cmd := exec.CommandContext(ctx, rgPath, args...)
	cmd.Dir = r.root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("prepare rg search: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("start rg search: %w", err)
	}

	results, total, parseErr := parseRGSearchOutput(stdout, query, limit)
	if parseErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, 0, ctx.Err()
	}
	if parseErr != nil {
		return nil, 0, fmt.Errorf("parse rg search output: %w", parseErr)
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 1 {
			return results, total, nil
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return nil, 0, fmt.Errorf("rg search failed: %s", message)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results, total, nil
}

type rgJSONEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgJSONPath struct {
	Text  string `json:"text"`
	Bytes string `json:"bytes"`
}

type rgJSONLines struct {
	Text  string `json:"text"`
	Bytes string `json:"bytes"`
}

type rgJSONSubmatch struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type rgJSONMatch struct {
	Path       rgJSONPath       `json:"path"`
	Lines      rgJSONLines      `json:"lines"`
	LineNumber int              `json:"line_number"`
	Submatches []rgJSONSubmatch `json:"submatches"`
}

func parseRGSearchOutput(reader io.Reader, query string, limit int) ([]domain.FileSearchResult, int, error) {
	lowerQuery := strings.ToLower(query)
	results := make([]domain.FileSearchResult, 0)
	resultIndexes := make(map[string]int)
	seenPaths := make(map[string]struct{})
	decoder := json.NewDecoder(reader)
	total := 0

	for {
		var event rgJSONEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, 0, err
		}
		if event.Type != "match" {
			continue
		}

		var match rgJSONMatch
		if err := json.Unmarshal(event.Data, &match); err != nil {
			return nil, 0, err
		}
		path, err := decodeRGText(match.Path.Text, match.Path.Bytes)
		if err != nil {
			return nil, 0, fmt.Errorf("decode matched path: %w", err)
		}
		path = filepath.ToSlash(strings.TrimPrefix(path, "./"))
		if path == "" {
			return nil, 0, errors.New("rg returned an empty matched path")
		}

		line := match.Lines.Text
		if line == "" && match.Lines.Bytes != "" {
			// The repository has historically skipped non-UTF-8 text and binary
			// files, so do the same for JSON output that contains encoded bytes.
			continue
		}
		if _, ok := seenPaths[path]; !ok {
			seenPaths[path] = struct{}{}
			total++
			result := domain.FileSearchResult{
				Path:       path,
				Line:       match.LineNumber,
				Snippet:    searchSnippet(line, lowerQuery),
				MatchCount: len(match.Submatches),
			}
			if limit <= 0 || len(results) < limit {
				resultIndexes[path] = len(results)
				results = append(results, result)
			} else {
				maxIndex := 0
				for index := 1; index < len(results); index++ {
					if results[maxIndex].Path < results[index].Path {
						maxIndex = index
					}
				}
				if path < results[maxIndex].Path {
					delete(resultIndexes, results[maxIndex].Path)
					resultIndexes[path] = maxIndex
					results[maxIndex] = result
				}
			}
			continue
		}

		if index, ok := resultIndexes[path]; ok {
			results[index].MatchCount += len(match.Submatches)
		}
	}

	return results, total, nil
}

func decodeRGText(text, encoded string) (string, error) {
	if encoded == "" {
		return text, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func searchableTextGlobs() []string {
	patterns := make([]string, 0, len(searchableTextExtensions)+len(searchableTextNames))
	for extension := range searchableTextExtensions {
		patterns = append(patterns, "*"+extension)
	}
	for name := range searchableTextNames {
		patterns = append(patterns, name)
	}
	sort.Strings(patterns)
	return patterns
}

func searchSnippet(line, lowerQuery string) string {
	const maxSnippetChars = 240
	line = strings.TrimSpace(line)
	lowerLine := strings.ToLower(line)
	if len([]rune(line)) <= maxSnippetChars {
		return line
	}
	index := strings.Index(lowerLine, lowerQuery)
	if index < 0 {
		return string([]rune(line)[:maxSnippetChars]) + "…"
	}
	runes := []rune(line)
	start := utf8.RuneCountInString(line[:index]) - 80
	if start < 0 {
		start = 0
	}
	end := start + maxSnippetChars
	if end > len(runes) {
		end = len(runes)
	}
	return func() string {
		snippet := string(runes[start:end])
		if start > 0 {
			snippet = "…" + snippet
		}
		if end < len(runes) {
			snippet += "…"
		}
		return snippet
	}()
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
		case kind == domain.FileKindFile && isMarkdownPath(d.Name()) && d.Type()&os.ModeSymlink == 0:
			status = markdownStatus(filepath.Join(dir, d.Name()))
		case kind == domain.FileKindDir && parent == "_tasks":
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
	if info.Size() > maxContentFileBytes {
		return "", fmt.Errorf("%w: %s exceeds the %d MiB content limit", domain.ErrResourceTooLarge, path, maxContentFileBytes>>20)
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
	if info.Size() > maxRawFileBytes {
		return nil, fmt.Errorf("%w: %s exceeds the %d MiB raw file limit", domain.ErrResourceTooLarge, path, maxRawFileBytes>>20)
	}
	return os.ReadFile(file)
}

func (r *FileRepository) Save(ctx context.Context, path string, content string) error {
	return r.SaveReader(ctx, path, strings.NewReader(content))
}

func (r *FileRepository) SaveBytes(ctx context.Context, path string, content []byte) error {
	return r.SaveReader(ctx, path, bytes.NewReader(content))
}

func (r *FileRepository) SaveReader(ctx context.Context, path string, content io.Reader) error {
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
	return fsutil.WriteFileAtomicReader(file, content, mode)
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
	var buf bytes.Buffer
	res, err := r.ExecuteStream(ctx, path, &buf)
	if err != nil {
		return domain.FileExecResult{}, err
	}
	res.Output = buf.String()
	return res, nil
}

func (r *FileRepository) executableFile(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
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
	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("%w: %s is not executable", domain.ErrInvalidPath, path)
	}
	return file, nil
}

func (r *FileRepository) ValidateExecutable(ctx context.Context, path string) error {
	_, err := r.executableFile(ctx, path)
	return err
}

func (r *FileRepository) ExecuteStream(ctx context.Context, path string, output io.Writer) (domain.FileExecResult, error) {
	file, err := r.executableFile(ctx, path)
	if err != nil {
		return domain.FileExecResult{}, err
	}
	ctx2, cancel := context.WithTimeout(ctx, executeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx2, file)
	if output == nil {
		output = io.Discard
	}
	writer := &lockedWriter{w: output}
	cmd.Stdout = writer
	cmd.Stderr = writer
	cmd.Dir = filepath.Dir(file)
	err = cmd.Run()
	res := domain.FileExecResult{}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		} else if ctx2.Err() == context.DeadlineExceeded {
			res.TimedOut = true
			res.ExitCode = 124
			if writer.wrote && writer.last != '\n' {
				_, _ = writer.Write([]byte{'\n'})
			}
			_, _ = writer.Write([]byte("[timed out after " + executeTimeout.String() + "]"))
		} else {
			return domain.FileExecResult{}, err
		}
	}
	return res, nil
}

type lockedWriter struct {
	mu    sync.Mutex
	w     io.Writer
	wrote bool
	last  byte
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.w.Write(p)
	if n > 0 {
		w.wrote = true
		w.last = p[n-1]
	}
	return n, err
}

func validateFilePath(path string) error {
	if path == "" || path == "." || path == ".." ||
		strings.HasPrefix(path, "/") || strings.Contains(path, "..") ||
		strings.ContainsAny(path, `\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, path)
	}
	return nil
}
