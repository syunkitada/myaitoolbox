package mygit

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml"
)

const (
	ManifestFilename = "mygit.yaml"
	LockFilename     = "mygit.lock.yaml"
	DefaultRepoDir   = "_repos"
)

var (
	ErrLockMissing = errors.New("lockfile is missing")
	ErrNoWorkspace = errors.New("no Workspace found")
)

type Manifest struct {
	Repositories []Repository `yaml:"repositories"`
}

type Repository struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Revision string `yaml:"revision"`
	Path     string `yaml:"path,omitempty"`
}

type Lockfile struct {
	Repositories []LockedRepository `yaml:"repositories"`
}

type LockedRepository struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Revision string `yaml:"revision"`
	Path     string `yaml:"path"`
	Commit   string `yaml:"commit"`
}

type Target struct {
	Repository Repository
	Path       string
	Relative   string
}

type Workspace struct {
	Root         string
	ManifestPath string
	Manifest     Manifest
	Targets      []Target
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	manifest, err = normalizeManifest(manifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return manifest, nil
}

func LoadLockfile(path string) (Lockfile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Lockfile{}, ErrLockMissing
	}
	if err != nil {
		return Lockfile{}, fmt.Errorf("read %s: %w", path, err)
	}
	var lock Lockfile
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return Lockfile{}, fmt.Errorf("parse %s: %w", path, err)
	}
	seen := make(map[string]struct{}, len(lock.Repositories))
	for _, repository := range lock.Repositories {
		if repository.Name == "" {
			return Lockfile{}, errors.New("lockfile repository name is required")
		}
		if _, ok := seen[repository.Name]; ok {
			return Lockfile{}, fmt.Errorf("duplicate lockfile repository name %q", repository.Name)
		}
		seen[repository.Name] = struct{}{}
		if !isObjectID(repository.Commit) {
			return Lockfile{}, fmt.Errorf("repository %q has an invalid full commit object ID", repository.Name)
		}
	}
	return lock, nil
}

func BuildWorkspace(root string, manifest Manifest, manifestPath string) (Workspace, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Workspace{}, fmt.Errorf("resolve Workspace root: %w", err)
	}
	manifest, err = normalizeManifest(manifest)
	if err != nil {
		return Workspace{}, err
	}
	workspace := Workspace{Root: filepath.Clean(absRoot), ManifestPath: manifestPath, Manifest: manifest}
	seenNames := make(map[string]struct{}, len(manifest.Repositories))
	seenTargets := make(map[string]string, len(manifest.Repositories))
	for _, repository := range manifest.Repositories {
		if _, ok := seenNames[repository.Name]; ok {
			return Workspace{}, fmt.Errorf("duplicate repository name %q", repository.Name)
		}
		seenNames[repository.Name] = struct{}{}

		target, err := resolveTarget(workspace.Root, repository)
		if err != nil {
			return Workspace{}, err
		}
		if previous, ok := seenTargets[target.Path]; ok {
			return Workspace{}, fmt.Errorf("repository %q collides with %q at %s", repository.Name, previous, target.Path)
		}
		seenTargets[target.Path] = repository.Name
		workspace.Targets = append(workspace.Targets, target)
	}
	return workspace, nil
}

func normalizeManifest(manifest Manifest) (Manifest, error) {
	for index, repository := range manifest.Repositories {
		if repository.Name == "" {
			name, err := deriveRepositoryName(repository.URL)
			if err != nil {
				return Manifest{}, fmt.Errorf("repository URL %q requires an explicit name: %w", repository.URL, err)
			}
			manifest.Repositories[index].Name = name
			repository.Name = name
		}
		if err := validateRepository(repository); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func validateRepository(repository Repository) error {
	if repository.Name == "" {
		return errors.New("repository name is required")
	}
	if !isSafeName(repository.Name) {
		return fmt.Errorf("repository name %q must be a single path element", repository.Name)
	}
	if repository.URL == "" {
		return fmt.Errorf("repository %q URL is required", repository.Name)
	}
	if repository.Revision == "" {
		return fmt.Errorf("repository %q revision is required", repository.Name)
	}
	return nil
}

func deriveRepositoryName(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("URL is empty")
	}

	var host string
	var escapedPath string
	if strings.Contains(rawURL, "://") {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return "", fmt.Errorf("parse URL: %w", err)
		}
		if parsed.Hostname() == "" || strings.EqualFold(parsed.Scheme, "file") {
			return "", errors.New("URL has no user or organization namespace")
		}
		host = parsed.Hostname()
		escapedPath = parsed.EscapedPath()
	} else if strings.HasPrefix(rawURL, "git@") {
		colon := strings.IndexByte(rawURL, ':')
		if colon <= len("git@") || colon == len(rawURL)-1 {
			return "", errors.New("SSH URL has no user or organization namespace")
		}
		host = rawURL[len("git@"):colon]
		escapedPath = rawURL[colon+1:]
	} else {
		return "", errors.New("URL has no user or organization namespace")
	}
	if host == "" {
		return "", errors.New("URL has no host")
	}

	parts := strings.Split(strings.Trim(escapedPath, "/"), "/")
	if len(parts) < 2 {
		return "", errors.New("URL path must contain a namespace and repository")
	}
	decoded := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return "", errors.New("URL path contains an empty component")
		}
		value, err := url.PathUnescape(part)
		if err != nil {
			return "", fmt.Errorf("decode URL path: %w", err)
		}
		if !isSafeName(value) {
			return "", fmt.Errorf("URL path component %q cannot be used in a name", value)
		}
		decoded = append(decoded, value)
	}
	repository := strings.TrimSuffix(decoded[len(decoded)-1], ".git")
	if repository == "" || !isSafeName(repository) {
		return "", errors.New("URL has no valid repository name")
	}
	decoded[len(decoded)-1] = repository
	name := strings.Join(decoded, "_")
	if !isSafeName(name) {
		return "", fmt.Errorf("derived name %q is not a safe path element", name)
	}
	return name, nil
}

func isSafeName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, character := range name {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func resolveTarget(root string, repository Repository) (Target, error) {
	relative := repository.Path
	if relative == "" {
		relative = filepath.Join(DefaultRepoDir, repository.Name)
	} else {
		relative = filepath.FromSlash(relative)
		if filepath.IsAbs(relative) {
			return Target{}, fmt.Errorf("repository %q path must be relative", repository.Name)
		}
		relative = filepath.Clean(relative)
		if relative == "." {
			return Target{}, fmt.Errorf("repository %q path cannot be the Workspace itself", repository.Name)
		}
	}

	targetPath, err := filepath.Abs(filepath.Join(root, relative))
	if err != nil {
		return Target{}, fmt.Errorf("resolve repository %q path: %w", repository.Name, err)
	}
	targetPath = filepath.Clean(targetPath)
	root = filepath.Clean(root)
	if targetPath == root {
		return Target{}, fmt.Errorf("repository %q path cannot be the Workspace itself", repository.Name)
	}
	reserved := filepath.Join(root, DefaultRepoDir)
	if isWithin(reserved, targetPath) && repository.Path != "" {
		return Target{}, fmt.Errorf("repository %q explicit path cannot be inside reserved %s", repository.Name, DefaultRepoDir)
	}
	normalized, err := filepath.Rel(root, targetPath)
	if err != nil {
		return Target{}, fmt.Errorf("normalize repository %q path: %w", repository.Name, err)
	}
	return Target{Repository: repository, Path: targetPath, Relative: filepath.ToSlash(normalized)}, nil
}

func ValidateLock(workspace Workspace, lock Lockfile) error {
	byName, err := lockRepositories(lock)
	if err != nil {
		return err
	}
	_, err = validateLockTargets(workspace, byName, false)
	return err
}

func missingLockTargets(workspace Workspace, lock Lockfile) ([]Target, error) {
	byName, err := lockRepositories(lock)
	if err != nil {
		return nil, err
	}
	return validateLockTargets(workspace, byName, true)
}

func lockRepositories(lock Lockfile) (map[string]LockedRepository, error) {
	byName := make(map[string]LockedRepository, len(lock.Repositories))
	for _, repository := range lock.Repositories {
		if _, ok := byName[repository.Name]; ok {
			return nil, fmt.Errorf("duplicate lockfile repository name %q", repository.Name)
		}
		if !isObjectID(repository.Commit) {
			return nil, fmt.Errorf("repository %q has an invalid full commit object ID", repository.Name)
		}
		byName[repository.Name] = repository
	}
	return byName, nil
}

func validateLockTargets(workspace Workspace, byName map[string]LockedRepository, allowMissing bool) ([]Target, error) {
	manifestNames := make(map[string]struct{}, len(workspace.Targets))
	for _, target := range workspace.Targets {
		manifestNames[target.Repository.Name] = struct{}{}
	}
	missing := make([]string, 0)
	for name := range manifestNames {
		if _, ok := byName[name]; !ok {
			missing = append(missing, name)
		}
	}
	extra := make([]string, 0)
	for name := range byName {
		if _, ok := manifestNames[name]; !ok {
			extra = append(extra, name)
		}
	}
	if len(extra) > 0 || (!allowMissing && len(missing) > 0) {
		return nil, fmt.Errorf(
			"manifest and lockfile repositories differ (manifest: %d, lockfile: %d; missing from lockfile: %s; extra in lockfile: %s)",
			len(workspace.Targets), len(byName), formatRepositoryNames(missing), formatRepositoryNames(extra),
		)
	}
	missingTargets := make([]Target, 0, len(missing))
	for _, target := range workspace.Targets {
		locked, ok := byName[target.Repository.Name]
		if !ok {
			missingTargets = append(missingTargets, target)
			continue
		}
		differences := make([]string, 0, 3)
		if locked.URL != target.Repository.URL {
			differences = append(differences, fmt.Sprintf("url: manifest=%q, lockfile=%q", target.Repository.URL, locked.URL))
		}
		if locked.Revision != target.Repository.Revision {
			differences = append(differences, fmt.Sprintf("revision: manifest=%q, lockfile=%q", target.Repository.Revision, locked.Revision))
		}
		if locked.Path != target.Relative {
			differences = append(differences, fmt.Sprintf("path: manifest=%q, lockfile=%q", target.Relative, locked.Path))
		}
		if len(differences) > 0 {
			return nil, fmt.Errorf("repository %q differs between manifest and lockfile: %s", target.Repository.Name, strings.Join(differences, "; "))
		}
	}
	return missingTargets, nil
}

func formatRepositoryNames(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func isObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isWithin(parent, path string) bool {
	relative, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func ValidateTargetCollisions(workspaces []Workspace) error {
	type owner struct {
		path       string
		workspace  string
		repository string
	}
	var owners []owner
	for _, workspace := range workspaces {
		for _, target := range workspace.Targets {
			canonical, err := canonicalPath(target.Path)
			if err != nil {
				return fmt.Errorf("resolve clone destination %s: %w", target.Path, err)
			}
			for _, previous := range owners {
				if isWithin(previous.path, canonical) || isWithin(canonical, previous.path) {
					return fmt.Errorf("clone destinations %s (%s/%s) and %s (%s/%s) overlap", previous.path, previous.workspace, previous.repository, canonical, workspace.Root, target.Repository.Name)
				}
			}
			owners = append(owners, owner{path: canonical, workspace: workspace.Root, repository: target.Repository.Name})
		}
	}
	return nil
}

func canonicalPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absPath = filepath.Clean(absPath)
	current := absPath
	var missing []string
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return absPath, nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
