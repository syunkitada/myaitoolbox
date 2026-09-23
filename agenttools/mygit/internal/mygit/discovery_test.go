package mygit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverWorkspacesPrunesCustomCloneDestinations(t *testing.T) {
	root := t.TempDir()
	manifest := `repositories:
  - name: custom
    url: https://example.test/custom.git
    revision: main
    path: ./custom
`
	if err := os.WriteFile(filepath.Join(root, ManifestFilename), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cloneManifestDir := filepath.Join(root, "custom")
	if err := os.MkdirAll(cloneManifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloneManifestDir, ManifestFilename), []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	workspaces, err := DiscoverWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("got %d workspaces, want 1", len(workspaces))
	}
	if workspaces[0].Root != root {
		t.Fatalf("got workspace %q, want %q", workspaces[0].Root, root)
	}
}
