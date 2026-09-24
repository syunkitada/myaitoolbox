package mygit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type GitRunner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

type CommandGit struct{}

func (CommandGit) Run(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		command.Dir = dir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return stdout.String(), fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, detail)
	}
	return stdout.String(), nil
}

func ResolveRevision(ctx context.Context, git GitRunner, url, revision string) (string, error) {
	if isObjectID(revision) {
		return strings.ToLower(revision), nil
	}
	if strings.ContainsAny(revision, "*?[]\\\r\n\t ") {
		return "", fmt.Errorf("revision %q contains invalid ref pattern characters", revision)
	}
	if _, err := git.Run(ctx, "", "check-ref-format", "--branch", revision); err != nil {
		return "", fmt.Errorf("invalid revision %q: %w", revision, err)
	}

	patterns := []string{}
	if strings.HasPrefix(revision, "refs/heads/") || strings.HasPrefix(revision, "refs/tags/") {
		patterns = append(patterns, revision)
		if strings.HasPrefix(revision, "refs/tags/") {
			patterns = append(patterns, revision+"^{}")
		}
	} else {
		patterns = append(patterns, "refs/heads/"+revision, "refs/tags/"+revision, "refs/tags/"+revision+"^{}")
	}
	args := append([]string{"ls-remote", url}, patterns...)
	out, err := git.Run(ctx, "", args...)
	if err != nil {
		return "", err
	}
	refs := parseRemoteRefs(out)
	branch := refs["refs/heads/"+revision]
	tag := refs["refs/tags/"+revision]
	peeled := refs["refs/tags/"+revision+"^{}"]
	if strings.HasPrefix(revision, "refs/heads/") {
		branch = refs[revision]
	}
	if strings.HasPrefix(revision, "refs/tags/") {
		tag = refs[revision]
		peeled = refs[revision+"^{}"]
	}
	if branch != "" && (tag != "" || peeled != "") {
		return "", fmt.Errorf("revision %q is ambiguous: both branch and tag exist", revision)
	}
	if branch != "" {
		return branch, nil
	}
	if peeled != "" {
		return peeled, nil
	}
	if tag != "" {
		return tag, nil
	}
	return "", fmt.Errorf("revision %q was not found in %s", revision, url)
}

func ResolveBranch(ctx context.Context, git GitRunner, url, revision string) (string, string, error) {
	branch, err := branchNameFromRevision(revision)
	if err != nil {
		return "", "", err
	}
	if _, err := git.Run(ctx, "", "check-ref-format", "--branch", branch); err != nil {
		return "", "", fmt.Errorf("invalid branch %q: %w", branch, err)
	}
	out, err := git.Run(ctx, "", "ls-remote", url, "refs/heads/"+branch)
	if err != nil {
		return "", "", err
	}
	commit := parseRemoteRefs(out)["refs/heads/"+branch]
	if commit == "" {
		return "", "", fmt.Errorf("branch %q was not found in %s", branch, url)
	}
	return branch, commit, nil
}

func branchNameFromRevision(revision string) (string, error) {
	if isObjectID(revision) {
		return "", fmt.Errorf("revision %q is a commit, not a branch", revision)
	}
	if strings.HasPrefix(revision, "refs/tags/") {
		return "", fmt.Errorf("revision %q is a tag, not a branch", revision)
	}
	revision = strings.TrimPrefix(revision, "refs/heads/")
	if strings.HasPrefix(revision, "refs/") || revision == "" {
		return "", fmt.Errorf("revision %q is not a branch name", revision)
	}
	if strings.ContainsAny(revision, "*?[]\\\r\n\t ") {
		return "", fmt.Errorf("revision %q contains invalid branch name characters", revision)
	}
	return revision, nil
}

func parseRemoteRefs(output string) map[string]string {
	refs := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !isObjectID(fields[0]) {
			continue
		}
		refs[fields[1]] = strings.ToLower(fields[0])
	}
	return refs
}
