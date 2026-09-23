package mygit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

type Service struct {
	Git GitRunner
}

func NewService() *Service {
	return &Service{Git: CommandGit{}}
}

func (s *Service) Sync(ctx context.Context, root string) error {
	return s.SyncWithOutput(ctx, root, io.Discard)
}

func (s *Service) SyncWithOutput(ctx context.Context, root string, output io.Writer) error {
	workspaces, err := DiscoverWorkspaces(root)
	if err != nil {
		return err
	}
	for _, workspace := range workspaces {
		report(output, "sync: workspace %s", workspace.Root)
		if err := s.syncWorkspace(ctx, workspace, output); err != nil {
			return fmt.Errorf("sync Workspace %s: %w", workspace.Root, err)
		}
		report(output, "sync: completed workspace %s", workspace.Root)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, root string) error {
	return s.UpdateWithOutput(ctx, root, io.Discard)
}

func (s *Service) UpdateWithOutput(ctx context.Context, root string, output io.Writer) error {
	workspaces, err := DiscoverWorkspaces(root)
	if err != nil {
		return err
	}
	for _, workspace := range workspaces {
		report(output, "update: workspace %s", workspace.Root)
		if err := s.updateWorkspace(ctx, workspace, output, "update"); err != nil {
			return fmt.Errorf("update Workspace %s: %w", workspace.Root, err)
		}
		report(output, "update: completed workspace %s", workspace.Root)
	}
	return nil
}

func (s *Service) Status(ctx context.Context, root string, output io.Writer) error {
	workspaces, err := DiscoverWorkspaces(root)
	if err != nil {
		return err
	}
	for _, workspace := range workspaces {
		if err := s.statusWorkspace(ctx, workspace, output); err != nil {
			return fmt.Errorf("status Workspace %s: %w", workspace.Root, err)
		}
	}
	return nil
}

func (s *Service) syncWorkspace(ctx context.Context, workspace Workspace, output io.Writer) error {
	lockPath := filepath.Join(workspace.Root, LockFilename)
	lock, err := LoadLockfile(lockPath)
	if errors.Is(err, ErrLockMissing) {
		report(output, "sync: lockfile missing; initializing %s", lockPath)
		return s.updateWorkspace(ctx, workspace, output, "sync")
	}
	if err != nil {
		return err
	}
	report(output, "sync: use lockfile %s", lockPath)
	discardedUnlocks := discardUnlockedRepositories(workspace, &lock)
	missing, err := missingLockTargets(workspace, lock)
	if err != nil {
		return err
	}
	for _, target := range missing {
		report(output, "sync: resolve missing %s (%s)", target.Repository.Name, target.Repository.Revision)
		commit, err := ResolveRevision(ctx, s.Git, target.Repository.URL, target.Repository.Revision)
		if err != nil {
			return fmt.Errorf("resolve missing %q: %w", target.Repository.Name, err)
		}
		report(output, "sync: resolved missing %s -> %s", target.Repository.Name, commit)
		lock.Repositories = append(lock.Repositories, LockedRepository{
			Name:     target.Repository.Name,
			URL:      target.Repository.URL,
			Revision: target.Repository.Revision,
			Path:     target.Relative,
			Commit:   commit,
		})
	}
	if changed, err := ensureGitignore(workspace); err != nil {
		return err
	} else if changed {
		report(output, "sync: update .gitignore %s", filepath.Join(workspace.Root, ".gitignore"))
	}
	locked := lockedByName(lock)
	for _, target := range workspace.Targets {
		commit := locked[target.Repository.Name].Commit
		if target.Repository.Unlock {
			report(output, "sync: resolve unlocked %s (%s)", target.Repository.Name, target.Repository.Revision)
			var err error
			commit, err = ResolveRevision(ctx, s.Git, target.Repository.URL, target.Repository.Revision)
			if err != nil {
				return fmt.Errorf("resolve unlocked %q: %w", target.Repository.Name, err)
			}
			report(output, "sync: resolved unlocked %s -> %s", target.Repository.Name, commit)
		}
		if err := s.syncTarget(ctx, target, commit, output, "sync"); err != nil {
			return err
		}
	}
	if len(missing) > 0 || discardedUnlocks {
		if err := WriteLockfile(lockPath, lock); err != nil {
			return err
		}
		report(output, "sync: write lockfile %s", lockPath)
	}
	return nil
}

func (s *Service) updateWorkspace(ctx context.Context, workspace Workspace, output io.Writer, operation string) error {
	locks := Lockfile{Repositories: make([]LockedRepository, 0, len(workspace.Targets))}
	commits := make(map[string]string, len(workspace.Targets))
	for _, target := range workspace.Targets {
		report(output, "%s: resolve %s (%s)", operation, target.Repository.Name, target.Repository.Revision)
		commit, err := ResolveRevision(ctx, s.Git, target.Repository.URL, target.Repository.Revision)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", target.Repository.Name, err)
		}
		report(output, "%s: resolved %s -> %s", operation, target.Repository.Name, commit)
		commits[target.Repository.Name] = commit
		if !target.Repository.Unlock {
			locks.Repositories = append(locks.Repositories, LockedRepository{
				Name:     target.Repository.Name,
				URL:      target.Repository.URL,
				Revision: target.Repository.Revision,
				Path:     target.Relative,
				Commit:   commit,
			})
		}
	}
	if changed, err := ensureGitignore(workspace); err != nil {
		return err
	} else if changed {
		report(output, "%s: update .gitignore %s", operation, filepath.Join(workspace.Root, ".gitignore"))
	}
	for _, target := range workspace.Targets {
		if err := s.syncTarget(ctx, target, commits[target.Repository.Name], output, operation); err != nil {
			return err
		}
	}
	lockPath := filepath.Join(workspace.Root, LockFilename)
	writeLock := len(locks.Repositories) > 0
	if !writeLock {
		if _, err := os.Stat(lockPath); err == nil {
			writeLock = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect lockfile %s: %w", lockPath, err)
		}
	}
	if writeLock {
		if err := WriteLockfile(lockPath, locks); err != nil {
			return err
		}
		report(output, "%s: write lockfile %s", operation, lockPath)
	}
	return nil
}

func (s *Service) syncTarget(ctx context.Context, target Target, commit string, output io.Writer, operation string) error {
	info, err := os.Lstat(target.Path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(target.Path), 0o755); err != nil {
			return fmt.Errorf("create parent for %q: %w", target.Repository.Name, err)
		}
		report(output, "%s: clone %s (%s) -> %s", operation, target.Repository.Name, target.Repository.URL, target.Path)
		if _, err := s.Git.Run(ctx, "", "clone", target.Repository.URL, target.Path); err != nil {
			return fmt.Errorf("clone %q: %w", target.Repository.Name, err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect clone destination %s: %w", target.Path, err)
	} else {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("clone destination %s is not a directory", target.Path)
		}
		if err := s.validateExistingTarget(ctx, target); err != nil {
			return err
		}
	}

	if err := s.validateExistingTarget(ctx, target); err != nil {
		return err
	}
	if err := s.ensureCommit(ctx, target.Path, commit, output, operation, target.Repository.Name); err != nil {
		return fmt.Errorf("prepare locked commit for %q: %w", target.Repository.Name, err)
	}
	report(output, "%s: checkout %s -> %s", operation, target.Repository.Name, commit)
	if _, err := s.Git.Run(ctx, target.Path, "checkout", "--detach", commit); err != nil {
		return fmt.Errorf("checkout commit for %q: %w", target.Repository.Name, err)
	}
	head, err := s.Git.Run(ctx, target.Path, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read HEAD for %q: %w", target.Repository.Name, err)
	}
	if !strings.EqualFold(strings.TrimSpace(head), commit) {
		return fmt.Errorf("repository %q is at %s, want %s", target.Repository.Name, strings.TrimSpace(head), commit)
	}
	return nil
}

func (s *Service) validateExistingTarget(ctx context.Context, target Target) error {
	top, err := s.Git.Run(ctx, target.Path, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("clone destination %s is not a Git worktree: %w", target.Path, err)
	}
	topPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(top)))
	if err != nil || topPath != filepath.Clean(target.Path) {
		return fmt.Errorf("clone destination %s is not an independent Git worktree", target.Path)
	}
	remote, err := s.Git.Run(ctx, target.Path, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("read remote for %q: %w", target.Repository.Name, err)
	}
	if strings.TrimSpace(remote) != target.Repository.URL {
		return fmt.Errorf("repository %q remote is %q, want %q", target.Repository.Name, strings.TrimSpace(remote), target.Repository.URL)
	}
	status, err := s.Git.Run(ctx, target.Path, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("read status for %q: %w", target.Repository.Name, err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("repository %q has uncommitted changes", target.Repository.Name)
	}
	return nil
}

func (s *Service) ensureCommit(ctx context.Context, target, commit string, output io.Writer, operation, name string) error {
	if _, err := s.Git.Run(ctx, target, "cat-file", "-e", commit+"^{commit}"); err == nil {
		return nil
	}
	report(output, "%s: fetch %s commit %s", operation, name, commit)
	if _, err := s.Git.Run(ctx, target, "fetch", "--no-tags", "origin", commit); err != nil {
		return err
	}
	if _, err := s.Git.Run(ctx, target, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return err
	}
	return nil
}

func lockedByName(lock Lockfile) map[string]LockedRepository {
	result := make(map[string]LockedRepository, len(lock.Repositories))
	for _, repository := range lock.Repositories {
		result[repository.Name] = repository
	}
	return result
}

func WriteLockfile(path string, lock Lockfile) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal lockfile: %w", err)
	}
	return atomicWrite(path, data, 0o644)
}

func ensureGitignore(workspace Workspace) (bool, error) {
	managed := false
	for _, target := range workspace.Targets {
		if target.Repository.Path == "" {
			managed = true
			break
		}
	}
	if !managed {
		return false, nil
	}
	path := filepath.Join(workspace.Root, ".gitignore")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = nil
	} else if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	updated, err := replaceManagedGitignore(string(data))
	if err != nil {
		return false, err
	}
	if string(data) == updated {
		return false, nil
	}
	if err := atomicWrite(path, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func report(output io.Writer, format string, args ...any) {
	if output == nil {
		return
	}
	_, _ = fmt.Fprintf(output, format+"\n", args...)
}

func replaceManagedGitignore(content string) (string, error) {
	const startMarker = "# mygit:start"
	const endMarker = "# mygit:end"
	const block = "# mygit:start\n/_repos/\n# mygit:end\n"
	start := strings.Index(content, startMarker)
	if start >= 0 {
		end := strings.Index(content[start+len(startMarker):], endMarker)
		if end < 0 {
			return "", errors.New(".gitignore contains an unterminated mygit managed block")
		}
		end += start + len(startMarker)
		if lineEnd := strings.IndexByte(content[end:], '\n'); lineEnd >= 0 {
			end += lineEnd + 1
		} else {
			end = len(content)
		}
		return content[:start] + block + content[end:], nil
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + block, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, ".mygit-*-")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryName)
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

type repoStatus struct {
	Target Target
	Locked string
	Remote string
	State  string
}

func (s *Service) statusWorkspace(ctx context.Context, workspace Workspace, output io.Writer) error {
	lock, err := LoadLockfile(filepath.Join(workspace.Root, LockFilename))
	if errors.Is(err, ErrLockMissing) {
		if workspaceHasLockedTargets(workspace) {
			_, _ = fmt.Fprintf(output, "workspace: %s\nstatus: invalid\nreason: lockfile is missing\n\n", workspace.Root)
			return nil
		}
		lock = Lockfile{}
		err = nil
	}
	if err != nil {
		_, _ = fmt.Fprintf(output, "workspace: %s\nstatus: invalid\nreason: %v\n\n", workspace.Root, err)
		return nil
	}
	if err := ValidateLock(workspace, lock); err != nil {
		_, _ = fmt.Fprintf(output, "workspace: %s\nstatus: invalid\nreason: %v\n\n", workspace.Root, err)
		return nil
	}
	locked := lockedByName(lock)
	statuses := make([]repoStatus, 0, len(workspace.Targets))
	for _, target := range workspace.Targets {
		entry := locked[target.Repository.Name]
		state := "locked"
		if target.Repository.Unlock {
			state = "unlocked"
		}
		if info, statErr := os.Stat(target.Path); errors.Is(statErr, os.ErrNotExist) {
			state = "missing"
		} else if statErr != nil || !info.IsDir() {
			state = "invalid"
		} else if err := s.validateExistingTarget(ctx, target); err != nil {
			if strings.Contains(err.Error(), "uncommitted changes") {
				state = "dirty"
			} else {
				state = "invalid"
			}
		} else if head, headErr := s.Git.Run(ctx, target.Path, "rev-parse", "HEAD"); headErr != nil {
			state = "invalid"
		} else if !target.Repository.Unlock && !strings.EqualFold(strings.TrimSpace(head), entry.Commit) {
			state = "drifted"
		}

		remote := "unavailable"
		if resolved, resolveErr := ResolveRevision(ctx, s.Git, target.Repository.URL, target.Repository.Revision); resolveErr == nil {
			remote = resolved
			if !target.Repository.Unlock && state == "locked" && !strings.EqualFold(resolved, entry.Commit) {
				state = "remote-outdated"
			}
		} else if state == "locked" {
			state = "remote-unavailable"
		}
		lockedCommit := entry.Commit
		if target.Repository.Unlock {
			lockedCommit = "-"
		}
		statuses = append(statuses, repoStatus{Target: target, Locked: lockedCommit, Remote: remote, State: state})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Target.Repository.Name < statuses[j].Target.Repository.Name })
	_, _ = fmt.Fprintf(output, "workspace: %s\n", workspace.Root)
	for _, status := range statuses {
		_, _ = fmt.Fprintf(output, "%s\n  revision: %s\n  locked:   %s\n  remote:   %s\n  status:   %s\n\n", status.Target.Repository.Name, status.Target.Repository.Revision, status.Locked, status.Remote, status.State)
	}
	return nil
}
