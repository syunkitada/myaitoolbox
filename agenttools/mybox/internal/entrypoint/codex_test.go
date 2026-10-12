package entrypoint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

func TestParseCodexDoctorUsesAllowlistedDetails(t *testing.T) {
	out := []byte(`{
  "overallStatus":"warning",
  "codexVersion":"0.162.1",
  "checks": {
    "auth.credentials": {"status":"ok","details": {
      "auth storage mode":"File",
      "stored API key":"false",
      "stored ChatGPT tokens":"true",
      "auth file":"/home/example/.codex/auth.json",
      "secret":"must not escape"
    }},
    "config.load": {"status":"ok","details": {
      "model":"gpt-5",
      "model provider":"openai",
      "config.toml":"/home/example/.codex/config.toml",
      "enabled feature flags":"shell_tool, hooks"
    }},
    "app_server.status": {"status":"ok","details": {
      "app-server version":"0.162.1",
      "control socket":"/home/example/.codex/control.sock"
    }}
  }
}`)

	runtime, err := codexRuntimeFromDoctor(out)
	require.NoError(t, err)
	assert.Equal(t, "warning", runtime.Status)
	assert.True(t, runtime.Available)
	assert.Equal(t, "0.162.1", *runtime.Version)
	assert.Equal(t, "0.162.1", *runtime.AppServerVersion)
	assert.Equal(t, "ok", *runtime.AppServerStatus)
	require.NotNil(t, runtime.AuthMode)
	assert.Equal(t, "File", *runtime.AuthMode)
	require.NotNil(t, runtime.Authenticated)
	assert.True(t, *runtime.Authenticated)
	assert.Equal(t, "gpt-5", *runtime.Model)
	assert.Equal(t, "openai", *runtime.ModelProvider)
	assert.NotContains(t, string(mustJSON(t, runtime)), "auth.json")
	assert.NotContains(t, string(mustJSON(t, runtime)), "secret")
}

func TestParseCodexDoctorReportsSafeStatusReasons(t *testing.T) {
	out := []byte(`{
  "overallStatus":"warning",
  "codexVersion":"0.162.1",
  "checks": {
    "state.rollout_db_parity": {"status":"warning","details": {
      "rollout DB missing active rows":"4",
      "rollout DB missing active sample":["/home/example/.codex/sessions/secret.jsonl"],
      "rollout DB archive mismatches":"0"
    }}
  }
}`)

	runtime, err := codexRuntimeFromDoctor(out)
	require.NoError(t, err)
	require.Len(t, runtime.StatusReasons, 1)
	assert.Contains(t, runtime.StatusReasons[0], "state.rollout_db_parity")
	assert.Contains(t, runtime.StatusReasons[0], "missing active rows=4")
	assert.Contains(t, runtime.StatusReasons[0], "missing active sample (1 item)")
	assert.NotContains(t, runtime.StatusReasons[0], "secret.jsonl")
	assert.NotContains(t, runtime.StatusReasons[0], "archive mismatches")
}

func TestParseCodexDoctorExplainsUnattributedOverallWarning(t *testing.T) {
	out := []byte(`{
  "overallStatus":"warning",
  "codexVersion":"0.162.1",
  "checks": {"config.load":{"status":"ok","details":{}}}
}`)

	runtime, err := codexRuntimeFromDoctor(out)
	require.NoError(t, err)
	require.Len(t, runtime.StatusReasons, 1)
	assert.Contains(t, runtime.StatusReasons[0], "no check-level reason")
}

func TestParseCodexDoctorReportsGenericSafeStatusDetails(t *testing.T) {
	out := []byte(`{
  "overallStatus":"warning",
  "codexVersion":"0.162.1",
  "checks": {
    "terminal.env": {"status":"warning","details": {
      "effective locale":"ja_JP.UTF-8",
      "terminal size":"80x24",
      "SSH_CLIENT":"192.0.2.1 12345",
      "auth token":"must not escape"
    }}
  }
}`)

	runtime, err := codexRuntimeFromDoctor(out)
	require.NoError(t, err)
	require.Len(t, runtime.StatusReasons, 1)
	assert.Contains(t, runtime.StatusReasons[0], "terminal.env")
	assert.Contains(t, runtime.StatusReasons[0], "effective locale")
	assert.Contains(t, runtime.StatusReasons[0], "terminal size")
	assert.NotContains(t, runtime.StatusReasons[0], "ja_JP.UTF-8")
	assert.NotContains(t, runtime.StatusReasons[0], "192.0.2.1")
	assert.NotContains(t, runtime.StatusReasons[0], "must not escape")
}

func TestParseCodexDoctorDoesNotTreatModelAsModeReason(t *testing.T) {
	out := []byte(`{
  "overallStatus":"warning",
  "codexVersion":"0.162.1",
  "checks": {
    "state.rollout_db_parity": {"status":"warning","details": {
      "default model provider":"openai",
      "rollout DB missing active rows":"4"
    }}
  }
}`)

	runtime, err := codexRuntimeFromDoctor(out)
	require.NoError(t, err)
	require.Len(t, runtime.StatusReasons, 1)
	assert.Contains(t, runtime.StatusReasons[0], "missing active rows=4")
	assert.NotContains(t, runtime.StatusReasons[0], "model provider")
}

func TestParseCodexMCPListDoesNotExposeArguments(t *testing.T) {
	out := []byte("Name        Command  Args                    Env  Cwd  Status   Auth\n" +
		"demo        node     --token=top-secret     -    -    enabled  Unsupported\n")

	servers := parseCodexMCPList(out)
	require.Len(t, servers, 1)
	assert.Equal(t, "demo", servers[0].Name)
	assert.Equal(t, "effective", servers[0].Scope)
	assert.Equal(t, "codex mcp list", servers[0].Source)
	assert.Equal(t, "node", servers[0].Command)
	assert.Equal(t, "stdio", servers[0].Transport)
	assert.Equal(t, "enabled", servers[0].Status)
	assert.Equal(t, "Unsupported", servers[0].AuthStatus)
	assert.Equal(t, "not_checked", servers[0].HealthStatus)
	assert.NotContains(t, string(mustJSON(t, servers[0])), "top-secret")

	servers = parseCodexMCPList([]byte("Name Command Args Env Cwd Status Auth\n" +
		"remote https://example.invalid/mcp?token=top-secret - - enabled token=top-secret\n"))
	require.Len(t, servers, 1)
	assert.Equal(t, "https://example.invalid/mcp", servers[0].Command)
	assert.Equal(t, "configured", servers[0].AuthStatus)
	assert.NotContains(t, string(mustJSON(t, servers[0])), "top-secret")

	servers = parseCodexMCPList([]byte("Name Command Args Env Cwd Status Auth\n" +
		"basic https://user:basic-secret@example.invalid/mcp - - enabled api-key=auth-secret\n"))
	require.Len(t, servers, 1)
	assert.Equal(t, "https://example.invalid/mcp", servers[0].Command)
	assert.Equal(t, "configured", servers[0].AuthStatus)
	assert.NotContains(t, string(mustJSON(t, servers[0])), "basic-secret")
	assert.NotContains(t, string(mustJSON(t, servers[0])), "auth-secret")
}

func TestCodexDiagnosticsReturnsPartialResultsWhenCommandsFail(t *testing.T) {
	s, _ := newTestServer(t)
	var calls [][]string
	s.codexRun = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		if len(calls) == 1 {
			return []byte(`{"overallStatus":"ok","codexVersion":"1.2.3","checks":{}}`), nil
		}
		return nil, assert.AnError
	}

	rec := do(t, s, http.MethodGet, "/api/codex/diagnostics", nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	var got api.CodexDiagnostics
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.Codex.Available)
	assert.Equal(t, "1.2.3", *got.Codex.Version)
	assert.NotEmpty(t, got.Errors)
	assert.Equal(t, [][]string{{"doctor", "--json"}, {"features", "list"}, {"mcp", "list"}}, calls)
}

func TestInspectCodexConfigFileReportsParseFailureWithoutReadingContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(`model = "gpt-5"\napi_key = "top-secret"\n`), 0o600))

	got := inspectCodexConfigFile(codexPathCandidate{scope: "user", path: path}, path, "error")
	assert.True(t, got.Exists)
	assert.True(t, got.Readable)
	assert.Equal(t, "invalid", got.Status)
	assert.NotContains(t, string(mustJSON(t, got)), "top-secret")
}

func TestRedactCodexConfigContentPreservesSafeValues(t *testing.T) {
	content := `# keep this comment
model = "gpt-5"
api_key = "top-secret"
"api_key" = "quoted-secret"
authToken = "camel-case-secret"
remote = "https://example.invalid/mcp?token=url-secret"
remote_basic = "https://user:config-secret@example.invalid/mcp"

[mcp_servers.demo]
command = "node"
args = ["--token", "args-secret", "--api-key=inline-secret"]
launch = "client --token launch-secret"
compound_args = ["--access-token", "access-token-secret", "--client-secret", "client-secret-value"]
args = ["--header", "X-API-Key: header-secret", "--header", "Authorization: header-auth"]

[mcp_servers.demo.env]
TOKEN = "env-secret"

[mcp_servers.demo.http_headers]
Authorization = "Bearer header-secret"

other = "Bearer bearer-secret"
cookie = "cookie-secret"
`

	got, redacted, err := redactCodexConfigContent([]byte(content))
	require.NoError(t, err)
	assert.True(t, redacted)
	assert.Contains(t, got, "# keep this comment")
	assert.Contains(t, got, `model = "gpt-5"`)
	assert.Contains(t, got, "api_key = \"[REDACTED]\"")
	assert.Contains(t, got, "\"api_key\" = \"[REDACTED]\"")
	assert.Contains(t, got, "authToken = \"[REDACTED]\"")
	assert.Contains(t, got, "TOKEN = \"[REDACTED]\"")
	assert.Contains(t, got, "Authorization = \"[REDACTED]\"")
	assert.NotContains(t, got, "top-secret")
	assert.NotContains(t, got, "quoted-secret")
	assert.NotContains(t, got, "camel-case-secret")
	assert.NotContains(t, got, "env-secret")
	assert.NotContains(t, got, "header-secret")
	assert.NotContains(t, got, "url-secret")
	assert.NotContains(t, got, "config-secret")
	assert.Contains(t, got, `remote_basic = "https://[REDACTED]@example.invalid/mcp"`)
	assert.NotContains(t, got, "bearer-secret")
	assert.NotContains(t, got, "cookie-secret")
	assert.NotContains(t, got, "args-secret")
	assert.NotContains(t, got, "inline-secret")
	assert.NotContains(t, got, "launch-secret")
	assert.NotContains(t, got, "access-token-secret")
	assert.NotContains(t, got, "client-secret-value")
	assert.NotContains(t, got, "header-secret")
	assert.NotContains(t, got, "header-auth")
}

func TestRedactCodexConfigContentMasksMultilineSensitiveValues(t *testing.T) {
	content := `multiline_token = [
  "array-secret",
]
`

	got, _, err := redactCodexConfigContent([]byte(content))
	require.NoError(t, err)
	assert.Contains(t, got, `multiline_token = "[REDACTED]"`)
	assert.NotContains(t, got, "array-secret")
}

func TestRedactCodexConfigContentMasksAuthHeaderValues(t *testing.T) {
	content := `args = ["--header", "X-Custom-Auth: custom-auth-secret"]
`

	got, _, err := redactCodexConfigContent([]byte(content))
	require.NoError(t, err)
	assert.Contains(t, got, `X-Custom-Auth: [REDACTED]`)
	assert.NotContains(t, got, "custom-auth-secret")
}

func TestGetCodexDiagnosticContentValidatesAndLimitsFiles(t *testing.T) {
	s, app := newTestServer(t)
	configPath := filepath.Join(app.Project.Path, ".codex", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`model = "gpt-5"
api_key = "top-secret"
`), 0o600))
	instructionPath := filepath.Join(app.Project.Path, "AGENTS.md")
	require.NoError(t, os.WriteFile(instructionPath, []byte("# project instructions\n"), 0o644))
	skillPath := filepath.Join(app.Project.Path, ".agents", "skills", "demo", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
	require.NoError(t, os.WriteFile(skillPath, []byte("---\nname: demo\n---\n\nSkill body\n"), 0o644))

	get := func(kind, path string) *httptest.ResponseRecorder {
		return do(t, s, http.MethodGet, "/api/codex/diagnostics/content?kind="+url.QueryEscape(kind)+"&path="+url.QueryEscape(path), nil, "X-Project", "test")
	}

	rec := get("config", configPath)
	assert.Equal(t, http.StatusOK, rec.Code)
	config := decode[api.CodexDiagnosticContent](t, rec)
	assert.Equal(t, "config", string(config.Kind))
	assert.Contains(t, config.Content, `model = "gpt-5"`)
	assert.NotContains(t, config.Content, "top-secret")
	assert.True(t, config.Redacted)
	assert.False(t, config.Truncated)

	rec = get("instruction", instructionPath)
	assert.Equal(t, http.StatusOK, rec.Code)
	instructions := decode[api.CodexDiagnosticContent](t, rec)
	assert.Equal(t, "# project instructions\n", instructions.Content)
	assert.False(t, instructions.Redacted)

	rec = get("skill", skillPath)
	assert.Equal(t, http.StatusOK, rec.Code)
	skill := decode[api.CodexDiagnosticContent](t, rec)
	assert.Contains(t, skill.Content, "Skill body")
	assert.False(t, skill.IsSymlink)

	linkedSkillTarget := filepath.Join(t.TempDir(), "SKILL.md")
	linkedSkillPath := filepath.Join(app.Project.Path, ".agents", "skills", "linked", "SKILL.md")
	require.NoError(t, os.WriteFile(linkedSkillTarget, []byte("---\nname: linked\n---\n\nLinked skill body\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Dir(linkedSkillPath), 0o755))
	require.NoError(t, os.Symlink(linkedSkillTarget, linkedSkillPath))

	rec = get("skill", linkedSkillPath)
	assert.Equal(t, http.StatusOK, rec.Code)
	linkedSkill := decode[api.CodexDiagnosticContent](t, rec)
	assert.Contains(t, linkedSkill.Content, "Linked skill body")
	assert.True(t, linkedSkill.IsSymlink)

	linkedSkillDirTarget := filepath.Join(t.TempDir(), "linked-skill")
	linkedNestedTarget := filepath.Join(linkedSkillDirTarget, "references", "guide.md")
	linkedSkillDirPath := filepath.Join(app.Project.Path, ".agents", "skills", "linked-directory")
	require.NoError(t, os.MkdirAll(filepath.Dir(linkedNestedTarget), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(linkedSkillDirTarget, "SKILL.md"), []byte("---\nname: linked-directory\n---\n"), 0o644))
	require.NoError(t, os.WriteFile(linkedNestedTarget, []byte("Linked nested guide\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Dir(linkedSkillDirPath), 0o755))
	require.NoError(t, os.Symlink(linkedSkillDirTarget, linkedSkillDirPath))

	rec = get("skill", filepath.Join(linkedSkillDirPath, "references", "guide.md"))
	assert.Equal(t, http.StatusOK, rec.Code)
	linkedNested := decode[api.CodexDiagnosticContent](t, rec)
	assert.Equal(t, "Linked nested guide\n", linkedNested.Content)
	assert.True(t, linkedNested.IsSymlink)

	rec = get("config", filepath.Join(app.Project.Path, "outside.toml"))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = get("config", app.Project.Path+string(filepath.Separator)+".codex"+string(filepath.Separator)+".."+string(filepath.Separator)+".codex"+string(filepath.Separator)+"config.toml")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetCodexDiagnosticContentAllowsUserConfigSymlink(t *testing.T) {
	s, _ := newTestServer(t)
	s.codexRun = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, assert.AnError
	}
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	targetPath := filepath.Join(t.TempDir(), "config.toml")
	configPath := filepath.Join(codexHome, "config.toml")
	require.NoError(t, os.WriteFile(targetPath, []byte("model = \"gpt-5\"\napi_key = \"outside-secret\"\n"), 0o600))
	require.NoError(t, os.Symlink(targetPath, configPath))

	target := "/api/codex/diagnostics/content?kind=config&path=" + url.QueryEscape(configPath)
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	content := decode[api.CodexDiagnosticContent](t, rec)
	assert.Equal(t, configPath, content.Path)
	assert.True(t, content.IsSymlink)
	assert.True(t, content.Redacted)
	assert.Contains(t, content.Content, `model = "gpt-5"`)
	assert.NotContains(t, content.Content, "outside-secret")

	rec = do(t, s, http.MethodGet, "/api/codex/diagnostics", nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	var diagnostics struct {
		ConfigFiles []map[string]any `json:"config_files"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &diagnostics))
	var configFile map[string]any
	for _, item := range diagnostics.ConfigFiles {
		if item["path"] == configPath {
			configFile = item
			break
		}
	}
	require.NotNil(t, configFile)
	assert.Equal(t, true, configFile["is_symlink"])
}

func TestProjectSkillsRootSymlinkIsBrowsable(t *testing.T) {
	s, app := newTestServer(t)
	projectSkillsRoot := filepath.Join(app.Project.Path, ".agents", "skills")
	targetSkillsRoot := filepath.Join(t.TempDir(), "skills")
	targetSkillDir := filepath.Join(targetSkillsRoot, "linked-root-skill")
	targetSkillPath := filepath.Join(targetSkillDir, "SKILL.md")
	targetReferencePath := filepath.Join(targetSkillDir, "references", "guide.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(projectSkillsRoot), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(targetReferencePath), 0o755))
	require.NoError(t, os.WriteFile(targetSkillPath, []byte("---\nname: linked-root-skill\n---\n\nRoot-linked skill body\n"), 0o644))
	require.NoError(t, os.WriteFile(targetReferencePath, []byte("Root-linked guide\n"), 0o644))
	require.NoError(t, os.Symlink(targetSkillsRoot, projectSkillsRoot))
	skillDir := filepath.Join(projectSkillsRoot, "linked-root-skill")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	rec := do(t, s, http.MethodGet, "/api/codex/diagnostics", nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	diagnostics := decode[api.CodexDiagnostics](t, rec)
	var skill api.CodexSkill
	for _, item := range diagnostics.Skills {
		if item.Path == skillPath {
			skill = item
			break
		}
	}
	assert.Equal(t, skillPath, skill.Path)
	assert.True(t, skill.IsSymlink)

	contentPath := "/api/codex/diagnostics/content?kind=skill&path=" + url.QueryEscape(skillPath)
	rec = do(t, s, http.MethodGet, contentPath, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	content := decode[api.CodexDiagnosticContent](t, rec)
	assert.Equal(t, "---\nname: linked-root-skill\n---\n\nRoot-linked skill body\n", content.Content)
	assert.True(t, content.IsSymlink)

	filesPath := "/api/codex/diagnostics/skill-files?path=" + url.QueryEscape(skillDir)
	rec = do(t, s, http.MethodGet, filesPath, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	listing := decode[api.CodexSkillFileList](t, rec)
	require.Len(t, listing.Entries, 2)
	assert.Equal(t, "references", listing.Entries[0].Name)
	assert.Equal(t, "SKILL.md", listing.Entries[1].Name)
}

func TestGetCodexSkillFilesListsDirectoryEntries(t *testing.T) {
	s, app := newTestServer(t)
	skillDir := filepath.Join(app.Project.Path, ".agents", "skills", "demo")
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "references", "guide.md"), []byte("guide\n"), 0o644))
	linkedFileTarget := filepath.Join(t.TempDir(), "linked.md")
	require.NoError(t, os.WriteFile(linkedFileTarget, []byte("linked\n"), 0o644))
	require.NoError(t, os.Symlink(linkedFileTarget, filepath.Join(skillDir, "linked.md")))

	target := "/api/codex/diagnostics/skill-files?path=" + url.QueryEscape(skillDir)
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	listing := decode[api.CodexSkillFileList](t, rec)
	assert.Equal(t, skillDir, listing.Path)
	require.Len(t, listing.Entries, 3)
	assert.False(t, listing.Truncated)
	assert.Equal(t, "references", listing.Entries[0].Name)
	assert.Equal(t, "SKILL.md", listing.Entries[1].Name)
	assert.Equal(t, "linked.md", listing.Entries[2].Name)
	assert.True(t, listing.Entries[2].IsSymlink)
	assert.Equal(t, linkedFileTarget, listing.Entries[2].Target)

	rec = do(t, s, http.MethodGet, "/api/codex/diagnostics/skill-files?path="+url.QueryEscape(filepath.Join(app.Project.Path, "outside")), nil, "X-Project", "test")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetCodexSkillFilesTruncatesLargeDirectory(t *testing.T) {
	s, app := newTestServer(t)
	skillDir := filepath.Join(app.Project.Path, ".agents", "skills", "large")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: large\n---\n"), 0o644))
	for i := 0; i < maxCodexSkillDirectoryEntries; i++ {
		path := filepath.Join(skillDir, "file-"+strconv.Itoa(i)+".md")
		require.NoError(t, os.WriteFile(path, []byte("content\n"), 0o644))
	}

	target := "/api/codex/diagnostics/skill-files?path=" + url.QueryEscape(skillDir)
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	listing := decode[api.CodexSkillFileList](t, rec)
	assert.Len(t, listing.Entries, maxCodexSkillDirectoryEntries)
	assert.True(t, listing.Truncated)
	foundSkillFile := false
	for _, entry := range listing.Entries {
		if entry.Name == "SKILL.md" {
			foundSkillFile = true
			break
		}
	}
	assert.True(t, foundSkillFile)
}

func TestGetCodexSkillFilesRejectsCyclicSkillDirectory(t *testing.T) {
	s, app := newTestServer(t)
	skillDir := filepath.Join(app.Project.Path, ".agents", "skills", "loop")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: loop\n---\n"), 0o644))
	require.NoError(t, os.Symlink(skillDir, filepath.Join(skillDir, "self")))

	target := "/api/codex/diagnostics/skill-files?path=" + url.QueryEscape(filepath.Join(skillDir, "self"))
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestScanCodexSkillsMarksSymlinks(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(t.TempDir(), "SKILL.md")
	link := filepath.Join(root, "linked", "SKILL.md")
	require.NoError(t, os.WriteFile(target, []byte("---\nname: linked\n---\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	require.NoError(t, os.Symlink(target, link))

	skills, truncated, err := scanCodexSkills(codexPathCandidate{scope: "user", path: root})
	require.NoError(t, err)
	require.Len(t, skills, 1)
	assert.False(t, truncated)
	assert.True(t, skills[0].IsSymlink)
}

func TestScanCodexSkillsLimitsDirectoryEnumeration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	require.NoError(t, os.MkdirAll(root, 0o755))
	for i := 0; i < maxCodexSkills+1; i++ {
		skillDir := filepath.Join(root, "skill-"+strconv.Itoa(i))
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: skill\n---\n"), 0o644))
	}

	skills, truncated, err := scanCodexSkills(codexPathCandidate{scope: "user", path: root})
	require.NoError(t, err)
	assert.Len(t, skills, maxCodexSkills)
	assert.True(t, truncated)
}

func TestFindCodexSkillDirectoryCandidateAllowsLinkedSkill(t *testing.T) {
	projectDir := t.TempDir()
	targetDir := filepath.Join(t.TempDir(), "linked")
	targetSkill := filepath.Join(targetDir, "SKILL.md")
	linkDir := filepath.Join(projectDir, ".agents", "skills", "linked")
	require.NoError(t, os.MkdirAll(filepath.Dir(targetSkill), 0o755))
	require.NoError(t, os.WriteFile(targetSkill, []byte("---\nname: linked\n---\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Dir(linkDir), 0o755))
	require.NoError(t, os.Symlink(targetDir, linkDir))

	_, ok := findCodexSkillDirectoryCandidate(projectDir, linkDir)
	assert.True(t, ok)
}

func TestGetCodexDiagnosticContentAllowsProjectSymlinkEscape(t *testing.T) {
	s, app := newTestServer(t)
	configDir := filepath.Join(app.Project.Path, ".codex")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "config.toml")
	require.NoError(t, os.WriteFile(outsidePath, []byte(`api_key = "outside-secret"\n`), 0o600))
	configPath := filepath.Join(configDir, "config.toml")
	require.NoError(t, os.Symlink(outsidePath, configPath))

	target := "/api/codex/diagnostics/content?kind=config&path=" + url.QueryEscape(configPath)
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	content := decode[api.CodexDiagnosticContent](t, rec)
	assert.True(t, content.IsSymlink)
	assert.True(t, content.Redacted)
	assert.NotContains(t, content.Content, "outside-secret")
}

func TestGetCodexDiagnosticContentAllowsProjectInstructionSymlink(t *testing.T) {
	s, app := newTestServer(t)
	s.codexRun = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, assert.AnError
	}
	outsidePath := filepath.Join(t.TempDir(), "AGENTS.md")
	instructionPath := filepath.Join(app.Project.Path, "AGENTS.md")
	require.NoError(t, os.WriteFile(outsidePath, []byte("# external instructions\n"), 0o644))
	require.NoError(t, os.Symlink(outsidePath, instructionPath))

	target := "/api/codex/diagnostics/content?kind=instruction&path=" + url.QueryEscape(instructionPath)
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	content := decode[api.CodexDiagnosticContent](t, rec)
	assert.Equal(t, "# external instructions\n", content.Content)
	assert.True(t, content.IsSymlink)

	rec = do(t, s, http.MethodGet, "/api/codex/diagnostics", nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	var diagnostics struct {
		Instructions []map[string]any `json:"instructions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &diagnostics))
	var instruction map[string]any
	for _, item := range diagnostics.Instructions {
		if item["path"] == instructionPath {
			instruction = item
			break
		}
	}
	require.NotNil(t, instruction)
	assert.Equal(t, true, instruction["is_symlink"])
}

func TestGetCodexDiagnosticContentAllowsSkillChildSymlink(t *testing.T) {
	s, app := newTestServer(t)
	skillDir := filepath.Join(app.Project.Path, ".agents", "skills", "demo")
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644))
	outsidePath := filepath.Join(t.TempDir(), "outside.md")
	require.NoError(t, os.WriteFile(outsidePath, []byte("outside-secret\n"), 0o644))
	linkedPath := filepath.Join(skillDir, "references", "outside.md")
	require.NoError(t, os.Symlink(outsidePath, linkedPath))

	target := "/api/codex/diagnostics/content?kind=skill&path=" + url.QueryEscape(linkedPath)
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	require.Equal(t, http.StatusOK, rec.Code)
	content := decode[api.CodexDiagnosticContent](t, rec)
	assert.Equal(t, "outside-secret\n", content.Content)
	assert.True(t, content.IsSymlink)
}

func TestGetCodexDiagnosticContentReportsTruncation(t *testing.T) {
	s, app := newTestServer(t)
	path := filepath.Join(app.Project.Path, "AGENTS.md")
	require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("instruction\n", maxCodexDiagnosticFileSize/len("instruction\n")+1)), 0o644))

	target := "/api/codex/diagnostics/content?kind=instruction&path=" + url.QueryEscape(path)
	rec := do(t, s, http.MethodGet, target, nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	content := decode[api.CodexDiagnosticContent](t, rec)
	assert.True(t, content.Truncated)
	assert.LessOrEqual(t, len(content.Content), maxCodexDiagnosticFileSize)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	out, err := json.Marshal(value)
	require.NoError(t, err)
	return out
}
