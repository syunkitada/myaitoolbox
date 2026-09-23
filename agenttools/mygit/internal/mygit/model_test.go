package mygit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestDoesNotRequireVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), ManifestFilename)
	data := []byte("repositories:\n  - name: example\n    url: https://example.test/repository.git\n    revision: main\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if len(manifest.Repositories) != 1 {
		t.Fatalf("loaded %d repositories, want 1", len(manifest.Repositories))
	}
}

func TestLoadLockfileDoesNotRequireVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFilename)
	data := []byte("repositories:\n  - name: example\n    url: https://example.test/repository.git\n    revision: main\n    path: _repos/example\n    commit: " + strings.Repeat("a", 40) + "\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	lock, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("LoadLockfile() error = %v", err)
	}
	if len(lock.Repositories) != 1 {
		t.Fatalf("loaded %d repositories, want 1", len(lock.Repositories))
	}
}

func TestValidateManifestRejectsUnsafeNamesAndReservedPaths(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name string
		path string
	}{
		{name: "../outside"},
		{name: "nested/name"},
		{name: "."},
		{name: ".."},
		{name: "inside-repos", path: "_repos/inside"},
		{name: "workspace", path: "."},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.path, func(t *testing.T) {
			manifest := Manifest{Repositories: []Repository{{
				Name:     tc.name,
				URL:      "https://example.test/repository.git",
				Revision: "main",
				Path:     tc.path,
			}}}
			if _, err := BuildWorkspace(root, manifest, filepath.Join(root, ManifestFilename)); err == nil {
				t.Fatalf("BuildWorkspace(%q, %q) succeeded", tc.name, tc.path)
			}
		})
	}
}

func TestBuildWorkspaceDerivesNameFromHTTPSURL(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{Repositories: []Repository{{
		URL:      "https://github.com/yennanliu/InvestSkill.git",
		Revision: "main",
	}}}
	workspace, err := BuildWorkspace(root, manifest, filepath.Join(root, ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := workspace.Manifest.Repositories[0].Name, "yennanliu_InvestSkill"; got != want {
		t.Fatalf("derived name = %q, want %q", got, want)
	}
	if got, want := workspace.Targets[0].Relative, "_repos/yennanliu_InvestSkill"; got != want {
		t.Fatalf("derived path = %q, want %q", got, want)
	}
}

func TestDeriveRepositoryNameSupportsSSHAndNestedNamespaces(t *testing.T) {
	cases := map[string]string{
		"git@github.com:yennanliu/InvestSkill.git":        "yennanliu_InvestSkill",
		"ssh://git@github.com/yennanliu/InvestSkill.git":  "yennanliu_InvestSkill",
		"https://gitlab.example/group/subgroup/repo.git":  "group_subgroup_repo",
		"https://gitlab.example/group/subgroup/repo.git/": "group_subgroup_repo",
	}
	for url, want := range cases {
		t.Run(url, func(t *testing.T) {
			got, err := deriveRepositoryName(url)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("derived name = %q, want %q", got, want)
			}
		})
	}
}

func TestDeriveRepositoryNameRequiresNamespace(t *testing.T) {
	for _, url := range []string{
		"file:///tmp/repository.git",
		"https://example.test/repository.git",
		"/tmp/repository.git",
	} {
		t.Run(url, func(t *testing.T) {
			if _, err := deriveRepositoryName(url); err == nil {
				t.Fatalf("deriveRepositoryName(%q) succeeded", url)
			}
		})
	}
}

func TestExplicitNameOverridesDerivedName(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{Repositories: []Repository{{
		Name:     "skills",
		URL:      "https://github.com/yennanliu/InvestSkill.git",
		Revision: "main",
	}}}
	workspace, err := BuildWorkspace(root, manifest, filepath.Join(root, ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := workspace.Manifest.Repositories[0].Name, "skills"; got != want {
		t.Fatalf("explicit name = %q, want %q", got, want)
	}
}

func TestValidateLockMatchesManifest(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{Repositories: []Repository{{
		Name:     "example",
		URL:      "https://example.test/repository.git",
		Revision: "main",
	}}}
	workspace, err := BuildWorkspace(root, manifest, filepath.Join(root, ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}

	lock := Lockfile{Repositories: []LockedRepository{{
		Name:     "example",
		URL:      manifest.Repositories[0].URL,
		Revision: manifest.Repositories[0].Revision,
		Path:     "_repos/example",
		Commit:   strings.Repeat("a", 40),
	}}}
	if err := ValidateLock(workspace, lock); err != nil {
		t.Fatalf("valid lock rejected: %v", err)
	}

	lock.Repositories[0].Revision = "v2"
	if err := ValidateLock(workspace, lock); err == nil {
		t.Fatal("manifest/lock revision drift was accepted")
	}
}

func TestValidateLockReportsRepositorySetDifferences(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{Repositories: []Repository{
		{Name: "first", URL: "https://example.test/first.git", Revision: "main"},
		{Name: "second", URL: "https://example.test/second.git", Revision: "main"},
	}}
	workspace, err := BuildWorkspace(root, manifest, filepath.Join(root, ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	lock := Lockfile{Repositories: []LockedRepository{{
		Name:     "first",
		URL:      manifest.Repositories[0].URL,
		Revision: manifest.Repositories[0].Revision,
		Path:     "_repos/first",
		Commit:   strings.Repeat("a", 40),
	}}}

	err = ValidateLock(workspace, lock)
	if err == nil {
		t.Fatal("manifest/lock repository set difference was accepted")
	}
	for _, detail := range []string{
		"manifest: 2, lockfile: 1",
		"missing from lockfile: second",
		"extra in lockfile: none",
	} {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("ValidateLock() error = %v, missing detail %q", err, detail)
		}
	}
}
