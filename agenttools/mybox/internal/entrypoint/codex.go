package entrypoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

const (
	codexDiagnosticsTimeout       = 15 * time.Second
	codexCommandTimeout           = 8 * time.Second
	maxCodexCommandOutput         = 2 << 20
	maxCodexSkillFileSize         = 64 << 10
	maxCodexDiagnosticFileSize    = 256 << 10
	maxCodexSkills                = 256
	maxCodexSkillDirectoryEntries = 512
)

var (
	codexSensitiveKeyPattern       = regexp.MustCompile(`(?i)(^|[._-])(secret|token|password|api[_-]?key|access[_-]?key|private[_-]?key|credential|authorization|auth|cookie|env|headers?|http[_-]?headers?|env[_-]?http[_-]?headers?)($|[._-])`)
	codexCamelCasePattern          = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	codexURLUserinfoPattern        = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)([^/?#\s"']+@)`)
	codexURLCredentialPattern      = regexp.MustCompile(`(?i)([?&](?:token|api[_-]?key|apikey|password|secret|credential|access[_-]?token|authorization|cookie)=)[^&#\s"']+`)
	codexBearerPattern             = regexp.MustCompile(`(?i)(\bBearer\s+)[A-Za-z0-9._~+/=-]+`)
	codexAPIKeyPattern             = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{8,}|gh[pousr]_[A-Za-z0-9_]{8,})\b`)
	codexCredentialKeyPattern      = `(?:(?:[a-z0-9]+[-_])?(?:token|secret|password|credential|authorization|auth|cookie|bearer)|(?:api|access|private)[-_]?key|apikey)`
	codexCLIArrayCredentialPattern = regexp.MustCompile(`(?i)(["'](?:--?|/)?` + codexCredentialKeyPattern + `["']\s*,\s*)(["'][^"']*["'])`)
	codexCLIFlagCredentialPattern  = regexp.MustCompile(`(?i)((?:--?|/)` + codexCredentialKeyPattern + `(?:=|:|\s+))("[^"]*"|'[^']*'|[^,\]\}\s"']+)`)
	codexBareCredentialPattern     = regexp.MustCompile(`(?i)(` + codexCredentialKeyPattern + `(?:=|:))("[^"]*"|'[^']*'|[^,\]\}\s"']+)`)
	codexHeaderCredentialPattern   = regexp.MustCompile(`(?i)(` + codexCredentialKeyPattern + `\s*[:=]\s*)([^,\]\}\r\n\s"'][^,\]\}\r\n"']*)`)
	codexStatusReasonKeyMarkers    = []string{"reason", "missing", "mismatch", "mismatches", "stale", "duplicate", "duplicates", "malformed", "scan", "failure", "failures", "failed", "warning", "warnings", "error", "errors", "status", "mode", "reachable", "available", "unavailable", "configured", "parse", "locale", "terminal", "stdin", "stdout", "stderr", "color"}
)

// codexRunFunc is kept injectable so diagnostics can be tested without
// invoking a user's CLI or contacting a provider.
type codexRunFunc func(ctx context.Context, projectDir string, args ...string) ([]byte, error)

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.exceeded = true
		return 0, errors.New("output limit exceeded")
	}
	return b.Buffer.Write(p)
}

func defaultCodexRun(ctx context.Context, projectDir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, codexCommandTimeout)
	defer cancel()

	bin, err := exec.LookPath("codex")
	if err != nil {
		return nil, errors.New("codex command not found")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = projectDir
	var stdout, stderr boundedBuffer
	stdout.limit = maxCodexCommandOutput
	stderr.limit = maxCodexCommandOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stdout.exceeded || stderr.exceeded {
			return nil, errors.New("codex diagnostic output exceeded limit")
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("codex diagnostic command timed out")
		}
		return nil, errors.New("codex diagnostic command failed")
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

func (s *Server) runCodex(ctx context.Context, projectDir string, args ...string) ([]byte, error) {
	if s.codexRun != nil {
		return s.codexRun(ctx, projectDir, args...)
	}
	return defaultCodexRun(ctx, projectDir, args...)
}

type rawCodexDoctor struct {
	OverallStatus string                   `json:"overallStatus"`
	CodexVersion  string                   `json:"codexVersion"`
	Checks        map[string]rawCodexCheck `json:"checks"`
}

type rawCodexCheck struct {
	Status  string                     `json:"status"`
	Details map[string]json.RawMessage `json:"details"`
}

func codexRuntimeFromDoctor(out []byte) (api.CodexRuntime, error) {
	var report rawCodexDoctor
	if err := json.Unmarshal(out, &report); err != nil {
		return api.CodexRuntime{}, errors.New("invalid Codex doctor report")
	}
	if report.Checks == nil {
		return api.CodexRuntime{}, errors.New("codex doctor report has no checks")
	}
	status := normalizeCodexStatus(report.OverallStatus)
	runtime := api.CodexRuntime{
		Status:        status,
		Available:     true,
		FeatureFlags:  splitCodexFeatureFlags(codexDetail(report, "config.load", "enabled feature flags")),
		StatusReasons: codexStatusReasons(report),
	}
	setCodexString(&runtime.Version, report.CodexVersion)
	setCodexString(&runtime.AppServerVersion, codexDetail(report, "app_server.status", "app-server version"))
	setCodexString(&runtime.AppServerStatus, codexCheckStatus(report, "app_server.status"))
	setCodexString(&runtime.AuthMode, firstCodexDetail(report, "auth.credentials", "stored auth mode", "auth storage mode", "auth mode"))
	setCodexString(&runtime.Model, codexDetail(report, "config.load", "model"))
	setCodexString(&runtime.ModelProvider, codexDetail(report, "config.load", "model provider"))
	setCodexString(&runtime.ApprovalPolicy, codexDetail(report, "sandbox.helpers", "approval policy"))
	setCodexString(&runtime.SandboxMode, codexDetail(report, "sandbox.helpers", "filesystem sandbox"))

	apiKey, apiKeyKnown := codexBoolDetail(report, "auth.credentials", "stored API key")
	tokens, tokensKnown := codexBoolDetail(report, "auth.credentials", "stored ChatGPT tokens")
	if apiKeyKnown || tokensKnown {
		authenticated := (apiKeyKnown && apiKey) || (tokensKnown && tokens)
		runtime.Authenticated = &authenticated
	}
	return runtime, nil
}

func normalizeCodexStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok", "warning", "error":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "warning"
	}
}

func codexStatusReasons(report rawCodexDoctor) []string {
	checkIDs := make([]string, 0, len(report.Checks))
	for checkID := range report.Checks {
		checkIDs = append(checkIDs, checkID)
	}
	sort.Strings(checkIDs)

	reasons := make([]string, 0)
	for _, checkID := range checkIDs {
		check := report.Checks[checkID]
		status := normalizeCodexStatus(check.Status)
		if status == "ok" {
			continue
		}
		reason := safeCodexDisplayValue(checkID) + " (" + status + ")"
		if details := codexStatusReasonDetails(check.Details); len(details) > 0 {
			reason += ": " + strings.Join(details, ", ")
		}
		reasons = append(reasons, safeCodexDisplayValue(reason))
	}
	if len(reasons) == 0 {
		overallStatus := normalizeCodexStatus(report.OverallStatus)
		if overallStatus != "ok" {
			reasons = append(reasons, "Codex doctor reported an overall "+overallStatus+" status but provided no check-level reason")
		}
	}
	return reasons
}

func codexStatusReasonDetails(details map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(details))
	for key := range details {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, 4)
	for _, key := range keys {
		isReasonKey := false
		for _, marker := range codexStatusReasonKeyMarkers {
			if codexStatusReasonKeyHasMarker(key, marker) {
				isReasonKey = true
				break
			}
		}
		if !isReasonKey {
			continue
		}

		var values []json.RawMessage
		if err := json.Unmarshal(details[key], &values); err == nil {
			if len(values) == 0 {
				continue
			}
			label := safeCodexDisplayValue(key)
			suffix := "items"
			if len(values) == 1 {
				suffix = "item"
			}
			result = append(result, label+" ("+strconv.Itoa(len(values))+" "+suffix+")")
		} else {
			var value string
			if json.Unmarshal(details[key], &value) != nil {
				continue
			}
			value = strings.TrimSpace(value)
			if value == "" || value == "false" || strings.EqualFold(value, "none") {
				continue
			}
			if number, err := strconv.ParseInt(value, 10, 64); err == nil && number == 0 {
				continue
			}
			// Keep the key and safe scalar counts, but never expose arbitrary
			// diagnostic values that may contain paths or credentials.
			if number, err := strconv.ParseInt(value, 10, 64); err == nil {
				result = append(result, safeCodexDisplayValue(key)+"="+strconv.FormatInt(number, 10))
			} else {
				result = append(result, safeCodexDisplayValue(key))
			}
		}
		if len(result) == 4 {
			break
		}
	}
	return result
}

func codexStatusReasonKeyHasMarker(key, marker string) bool {
	words := strings.FieldsFunc(strings.ToLower(key), func(r rune) bool {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return false
		}
		return true
	})
	for _, word := range words {
		if word == marker {
			return true
		}
	}
	return false
}

func codexDetail(report rawCodexDoctor, checkID, key string) string {
	check, ok := report.Checks[checkID]
	if !ok {
		return ""
	}
	value, ok := check.Details[key]
	if !ok {
		return ""
	}
	var result string
	if json.Unmarshal(value, &result) != nil {
		return ""
	}
	return safeCodexDisplayValue(result)
}

func codexCheckStatus(report rawCodexDoctor, checkID string) string {
	check, ok := report.Checks[checkID]
	if !ok {
		return ""
	}
	return normalizeCodexStatus(check.Status)
}

func firstCodexDetail(report rawCodexDoctor, checkID string, keys ...string) string {
	for _, key := range keys {
		if value := codexDetail(report, checkID, key); value != "" {
			return value
		}
	}
	return ""
}

func codexBoolDetail(report rawCodexDoctor, checkID, key string) (bool, bool) {
	value := strings.ToLower(codexDetail(report, checkID, key))
	switch value {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func setCodexString(dst **string, value string) {
	if value == "" {
		return
	}
	value = safeCodexDisplayValue(value)
	if value != "" {
		*dst = &value
	}
}

func safeCodexDisplayValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return -1
	}, value)
	if len(value) > 256 {
		return value[:256]
	}
	return value
}

func splitCodexFeatureFlags(value string) []string {
	if value == "" || value == "none" {
		return []string{}
	}
	flags := make([]string, 0, 16)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" && len(flags) < 256 {
			flags = append(flags, item)
		}
	}
	sort.Strings(flags)
	return flags
}

func parseCodexFeatureList(out []byte) []string {
	features := make([]string, 0, 32)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.ToLower(fields[len(fields)-1]) != "true" {
			continue
		}
		name := safeCodexDisplayValue(fields[0])
		if name != "" && len(features) < 256 {
			features = append(features, name)
		}
	}
	sort.Strings(features)
	return features
}

func parseCodexMCPList(out []byte) []api.CodexMCPServer {
	servers := make([]api.CodexMCPServer, 0)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || strings.EqualFold(fields[0], "name") {
			continue
		}
		name := safeCodexDisplayValue(fields[0])
		command := redactCodexCommand(safeCodexDisplayValue(fields[1]))
		if name == "" {
			continue
		}
		status := safeCodexDisplayValue(fields[len(fields)-2])
		auth := sanitizeCodexAuthStatus(fields[len(fields)-1])
		transport := "stdio"
		if parsed, err := url.Parse(command); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			transport = "http"
		}
		servers = append(servers, api.CodexMCPServer{
			Name:         name,
			Scope:        "effective",
			Source:       "codex mcp list",
			Transport:    transport,
			Command:      command,
			Status:       status,
			AuthStatus:   auth,
			HealthStatus: "not_checked",
		})
	}
	return servers
}

func sanitizeCodexAuthStatus(value string) string {
	value = safeCodexDisplayValue(value)
	if value == "" || value == "-" {
		return "none"
	}
	lower := strings.ToLower(value)
	normalized := strings.NewReplacer("-", "_", " ", "_").Replace(lower)
	for _, marker := range []string{"token", "api_key", "apikey", "password", "secret", "bearer", "access_key", "private_key", "credential", "authorization", "cookie"} {
		if strings.Contains(normalized, marker) {
			return "configured"
		}
	}
	if len(value) > 64 {
		return "configured"
	}
	return value
}

func redactCodexCommand(command string) string {
	if parsed, err := url.Parse(command); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		parsed.Fragment = ""
		return safeCodexDisplayValue(parsed.String())
	}
	lower := strings.ToLower(command)
	for _, marker := range []string{"token=", "api_key=", "apikey=", "password=", "secret="} {
		if index := strings.Index(lower, marker); index >= 0 {
			return command[:index] + marker + "<redacted>"
		}
	}
	return command
}

type codexPathCandidate struct {
	scope string
	path  string
}

func codexHomeDir() string {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return filepath.Clean(home)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".codex")
}

func addCodexPathCandidate(candidates *[]codexPathCandidate, seen map[string]bool, scope, path string) {
	if path == "" {
		return
	}
	path = filepath.Clean(path)
	if seen[path] {
		return
	}
	seen[path] = true
	*candidates = append(*candidates, codexPathCandidate{scope: scope, path: path})
}

func codexConfigCandidates(projectDir string) []codexPathCandidate {
	var candidates []codexPathCandidate
	seen := make(map[string]bool)
	if home := codexHomeDir(); home != "" {
		addCodexPathCandidate(&candidates, seen, "user", filepath.Join(home, "config.toml"))
	}
	addCodexPathCandidate(&candidates, seen, "project", filepath.Join(projectDir, ".codex", "config.toml"))
	addCodexPathCandidate(&candidates, seen, "admin", "/etc/codex/config.toml")
	return candidates
}

func inspectCodexConfigFile(candidate codexPathCandidate, loadedPath, parseStatus string) api.CodexConfigFile {
	result := api.CodexConfigFile{Scope: candidate.scope, Path: candidate.path, Status: "missing"}
	info, err := os.Stat(candidate.path)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Status = "unavailable"
		}
		return result
	}
	result.Exists = true
	if info.IsDir() {
		result.Status = "invalid"
		return result
	}
	file, err := os.Open(candidate.path)
	if err != nil {
		result.Status = "unreadable"
		return result
	}
	_ = file.Close()
	result.Readable = true
	result.IsSymlink = codexPathIsSymlink(candidate.path)
	result.Status = "present"
	if loadedPath != "" && filepath.Clean(loadedPath) == candidate.path {
		if strings.EqualFold(parseStatus, "ok") || parseStatus == "" {
			result.Status = "loaded"
		} else {
			result.Status = "invalid"
		}
	}
	return result
}

func readLimitedFile(path string, maxSize int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, int64(maxSize)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSize {
		return nil, errors.New("file is too large")
	}
	return data, nil
}

type codexContentCandidate struct {
	scope     string
	path      string
	root      string
	isSymlink bool
}

func codexContentCandidates(projectDir string, kind api.GetCodexDiagnosticContentParamsKind) []codexContentCandidate {
	projectDir = filepath.Clean(projectDir)
	result := make([]codexContentCandidate, 0)
	switch kind {
	case api.GetCodexDiagnosticContentParamsKindConfig:
		for _, candidate := range codexConfigCandidates(projectDir) {
			root := filepath.Dir(candidate.path)
			switch candidate.scope {
			case "project":
				root = filepath.Join(projectDir, ".codex")
			case "admin":
				root = "/etc/codex"
			}
			result = append(result, codexContentCandidate{scope: candidate.scope, path: candidate.path, root: root})
		}
	case api.GetCodexDiagnosticContentParamsKindInstruction:
		projectRoot := projectRootForCodex(projectDir)
		for _, candidate := range codexInstructionCandidates(projectDir) {
			root := codexHomeDir()
			if candidate.scope == "project" {
				root = projectRoot
			}
			result = append(result, codexContentCandidate{scope: candidate.scope, path: candidate.path, root: root})
		}
	}
	return result
}

func pathWithinCodexRoot(path, root string) bool {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(resolvedRoot), filepath.Clean(resolvedPath))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func codexPathIsSymlink(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return filepath.Clean(resolved) != filepath.Clean(path)
}

func codexContentCandidateAllowed(candidate codexContentCandidate, projectDir string) bool {
	if !filepath.IsAbs(candidate.path) {
		return false
	}
	if candidate.scope != "project" {
		// User/global and admin candidates are fixed, discovered paths. Their
		// targets may intentionally live outside the Codex directory (for
		// example, a user-managed config symlink), and the response still
		// reports the lexical path as a symlink to the caller.
		return true
	}
	projectRoot := projectRootForCodex(projectDir)
	if pathWithinCodexRoot(candidate.path, candidate.root) && pathWithinCodexRoot(candidate.root, projectRoot) {
		return true
	}
	// A project-level Config or AGENTS.md may intentionally be a symlink to a
	// user-managed file outside the project. The candidate itself is still
	// fixed by Codex discovery and must remain lexically under the project
	// root; arbitrary paths are not accepted here.
	return codexPathIsSymlink(candidate.path) && pathWithinCodexLexicalRoot(candidate.path, projectRoot)
}

func pathWithinCodexLexicalRoot(path, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func codexSkillCandidateAllowed(candidate codexContentCandidate, projectDir string) bool {
	if !filepath.IsAbs(candidate.path) || !filepath.IsAbs(candidate.root) {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(candidate.root), filepath.Clean(candidate.path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 2 || parts[0] == "" || parts[1] != "SKILL.md" {
		return false
	}
	if candidate.scope == "project" {
		projectRoot := projectRootForCodex(projectDir)
		// The standard project Skills roots are allowed to point at a
		// user-managed external directory. Keep the root itself restricted to
		// the project tree lexically, while validating child links against the
		// resolved skill directory below.
		if !pathWithinCodexRoot(candidate.root, projectRoot) && !pathWithinCodexLexicalRoot(candidate.root, projectRoot) {
			return false
		}
	}
	info, err := os.Stat(candidate.path)
	return err == nil && info.Mode().IsRegular()
}

func codexSkillPathCandidate(projectDir, requestedPath string) (codexPathCandidate, string, bool) {
	cleanPath := filepath.Clean(requestedPath)
	if requestedPath == "" || !filepath.IsAbs(requestedPath) || requestedPath != cleanPath {
		return codexPathCandidate{}, "", false
	}
	for _, root := range codexSkillRoots(projectDir) {
		rel, err := filepath.Rel(filepath.Clean(root.path), requestedPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 1 || parts[0] == "" || parts[0] == "." {
			continue
		}
		skillDir := filepath.Join(root.path, parts[0])
		skillPath := filepath.Join(skillDir, "SKILL.md")
		skillCandidate := codexContentCandidate{scope: root.scope, path: skillPath, root: root.path}
		if !codexSkillCandidateAllowed(skillCandidate, projectDir) {
			continue
		}
		skillRel, err := filepath.Rel(filepath.Clean(skillDir), requestedPath)
		if err != nil || skillRel == ".." || strings.HasPrefix(skillRel, ".."+string(filepath.Separator)) {
			continue
		}
		return root, skillDir, true
	}
	return codexPathCandidate{}, "", false
}

func findCodexSkillFileCandidate(projectDir, requestedPath string) (codexContentCandidate, bool) {
	root, _, ok := codexSkillPathCandidate(projectDir, requestedPath)
	if !ok {
		return codexContentCandidate{}, false
	}
	info, err := os.Stat(requestedPath)
	if err != nil || !info.Mode().IsRegular() {
		return codexContentCandidate{}, false
	}
	candidate := codexContentCandidate{scope: root.scope, path: requestedPath, root: root.path}
	candidate.isSymlink = codexPathIsSymlink(candidate.path)
	return candidate, true
}

type codexSkillDirectoryCandidate struct {
	scope     string
	path      string
	root      string
	isSymlink bool
}

func findCodexSkillDirectoryCandidate(projectDir, requestedPath string) (codexSkillDirectoryCandidate, bool) {
	root, skillDir, ok := codexSkillPathCandidate(projectDir, requestedPath)
	if !ok {
		return codexSkillDirectoryCandidate{}, false
	}
	info, err := os.Stat(requestedPath)
	if err != nil || !info.IsDir() {
		return codexSkillDirectoryCandidate{}, false
	}
	if requestedPath != skillDir && codexPathIsSymlink(requestedPath) {
		resolvedPath, pathErr := filepath.EvalSymlinks(requestedPath)
		resolvedSkillDir, rootErr := filepath.EvalSymlinks(skillDir)
		if pathErr != nil || rootErr != nil {
			return codexSkillDirectoryCandidate{}, false
		}
		rel, relErr := filepath.Rel(filepath.Clean(resolvedSkillDir), filepath.Clean(resolvedPath))
		if relErr != nil || rel == "." {
			return codexSkillDirectoryCandidate{}, false
		}
	}
	return codexSkillDirectoryCandidate{
		scope:     root.scope,
		path:      requestedPath,
		root:      root.path,
		isSymlink: codexPathIsSymlink(requestedPath),
	}, true
}

func findCodexContentCandidate(projectDir string, kind api.GetCodexDiagnosticContentParamsKind, requestedPath string) (codexContentCandidate, bool) {
	cleanPath := filepath.Clean(requestedPath)
	if requestedPath == "" || !filepath.IsAbs(requestedPath) || requestedPath != cleanPath {
		return codexContentCandidate{}, false
	}
	requestedPath = cleanPath
	if kind == api.GetCodexDiagnosticContentParamsKindSkill {
		return findCodexSkillFileCandidate(projectDir, requestedPath)
	}
	for _, candidate := range codexContentCandidates(projectDir, kind) {
		if candidate.path == requestedPath && codexContentCandidateAllowed(candidate, projectDir) {
			candidate.isSymlink = codexPathIsSymlink(candidate.path)
			return candidate, true
		}
	}
	return codexContentCandidate{}, false
}

func readCodexDiagnosticFile(path string) ([]byte, int64, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, false, errors.New("diagnostic content is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, int64(maxCodexDiagnosticFileSize)+1))
	if err != nil {
		return nil, 0, false, err
	}
	truncated := len(data) > maxCodexDiagnosticFileSize || info.Size() > maxCodexDiagnosticFileSize
	if len(data) > maxCodexDiagnosticFileSize {
		data = data[:maxCodexDiagnosticFileSize]
	}
	if truncated {
		if lastNewline := bytes.LastIndexByte(data, '\n'); lastNewline >= 0 {
			data = data[:lastNewline+1]
		}
	}
	if !utf8.Valid(data) {
		if !truncated {
			return nil, 0, false, errors.New("diagnostic content is not valid UTF-8")
		}
		for len(data) > 0 && data[len(data)-1] >= utf8.RuneSelf && !utf8.Valid(data) {
			data = data[:len(data)-1]
		}
		if !utf8.Valid(data) {
			return nil, 0, false, errors.New("diagnostic content is not valid UTF-8")
		}
	}
	return data, info.Size(), truncated, nil
}

func codexSymlinkTarget(path string) string {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		return target
	}
	if target, err := os.Readlink(path); err == nil {
		return target
	}
	return ""
}

func codexSkillFileKind(info os.FileInfo, err error) string {
	if err != nil {
		return "unavailable"
	}
	if info.IsDir() {
		return "directory"
	}
	if info.Mode().IsRegular() {
		return "file"
	}
	return "other"
}

func readCodexSkillDirectory(path string) ([]api.CodexSkillFile, bool, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = directory.Close() }()
	names, err := directory.Readdirnames(maxCodexSkillDirectoryEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	truncated := len(names) > maxCodexSkillDirectoryEntries
	if truncated {
		names = names[:maxCodexSkillDirectoryEntries]
		hasSkillFile := false
		for _, name := range names {
			if name == "SKILL.md" {
				hasSkillFile = true
				break
			}
		}
		if !hasSkillFile {
			if _, statErr := os.Lstat(filepath.Join(path, "SKILL.md")); statErr == nil {
				names[len(names)-1] = "SKILL.md"
			}
		}
	}
	result := make([]api.CodexSkillFile, 0, len(names))
	for _, name := range names {
		entryPath := filepath.Join(path, name)
		linkInfo, linkErr := os.Lstat(entryPath)
		if linkErr != nil {
			continue
		}
		info, statErr := os.Stat(entryPath)
		size := int64(0)
		if statErr == nil {
			size = info.Size()
		}
		target := ""
		isSymlink := linkInfo.Mode()&os.ModeSymlink != 0
		if isSymlink {
			target = codexSymlinkTarget(entryPath)
		}
		result = append(result, api.CodexSkillFile{
			Name:      name,
			Path:      entryPath,
			Kind:      codexSkillFileKind(info, statErr),
			SizeBytes: size,
			IsSymlink: isSymlink,
			Target:    target,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		rank := func(kind string) int {
			if kind == "directory" {
				return 0
			}
			if kind == "file" {
				return 1
			}
			return 2
		}
		if rank(result[i].Kind) != rank(result[j].Kind) {
			return rank(result[i].Kind) < rank(result[j].Kind)
		}
		return result[i].Name < result[j].Name
	})
	return result, truncated, nil
}

func findCodexAssignmentSeparator(line string) int {
	var quote rune
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if char == '\'' || char == '"' {
			switch quote {
			case 0:
				quote = char
			case char:
				quote = 0
			}
			continue
		}
		if char == '=' && quote == 0 {
			return index
		}
	}
	return -1
}

func findCodexCommentStart(value string) int {
	var quote rune
	escaped := false
	for index, char := range value {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if char == '\'' || char == '"' {
			switch quote {
			case 0:
				quote = char
			case char:
				quote = 0
			}
			continue
		}
		if char == '#' && quote == 0 {
			return index
		}
	}
	return -1
}

func codexBracketDelta(value string) int {
	var quote rune
	escaped := false
	delta := 0
	for _, char := range value {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if char == '\'' || char == '"' {
			switch quote {
			case 0:
				quote = char
			case char:
				quote = 0
			}
			continue
		}
		if quote == 0 {
			if char == '#' {
				break
			}
			switch char {
			case '[':
				delta++
			case ']':
				delta--
			}
		}
	}
	return delta
}

func isSensitiveCodexSection(section string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(section, "\"", ""))
	if !strings.Contains(normalized, "mcp_servers.") {
		return false
	}
	return strings.HasSuffix(normalized, ".env") ||
		strings.HasSuffix(normalized, ".headers") ||
		strings.HasSuffix(normalized, ".http_headers") ||
		strings.HasSuffix(normalized, ".env_http_headers")
}

func isSensitiveCodexKey(key string) bool {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"'")
	key = codexCamelCasePattern.ReplaceAllString(key, `${1}_${2}`)
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "\"", ""), "'", ""))
	return codexSensitiveKeyPattern.MatchString(key)
}

func redactCodexGenericCredentials(value string) string {
	value = codexURLUserinfoPattern.ReplaceAllString(value, `${1}[REDACTED]@`)
	value = codexURLCredentialPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	value = codexBearerPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	value = codexHeaderCredentialPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	value = codexCLIArrayCredentialPattern.ReplaceAllString(value, `${1}"[REDACTED]"`)
	value = codexCLIFlagCredentialPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	value = codexBareCredentialPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	return codexAPIKeyPattern.ReplaceAllString(value, "[REDACTED]")
}

func redactCodexConfigContent(data []byte) (string, bool, error) {
	if !utf8.Valid(data) {
		return "", false, errors.New("config content is not valid UTF-8")
	}
	var builder strings.Builder
	section := ""
	maskedMultiline := ""
	maskedArrayDepth := 0
	for _, rawLine := range strings.SplitAfter(string(data), "\n") {
		line := strings.TrimSuffix(rawLine, "\n")
		newline := ""
		if strings.HasSuffix(rawLine, "\n") {
			newline = "\n"
		}
		if maskedMultiline != "" {
			if strings.Contains(line, maskedMultiline) {
				maskedMultiline = ""
			}
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			builder.WriteString(indent + `"[REDACTED]"` + newline)
			continue
		}
		if maskedArrayDepth > 0 {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			builder.WriteString(indent + `"[REDACTED]"` + newline)
			maskedArrayDepth += codexBracketDelta(line)
			if maskedArrayDepth <= 0 {
				maskedArrayDepth = 0
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]") {
			end := strings.Index(trimmed, "]")
			section = strings.TrimSpace(strings.Trim(trimmed[1:end], "[]"))
		}
		separator := findCodexAssignmentSeparator(line)
		if separator < 0 {
			builder.WriteString(redactCodexGenericCredentials(line) + newline)
			continue
		}
		key := strings.TrimSpace(line[:separator])
		valueWithComment := line[separator+1:]
		commentStart := findCodexCommentStart(valueWithComment)
		value := valueWithComment
		comment := ""
		if commentStart >= 0 {
			value = valueWithComment[:commentStart]
			comment = valueWithComment[commentStart:]
		}
		mask := isSensitiveCodexKey(key) || isSensitiveCodexSection(section)
		if !mask {
			builder.WriteString(redactCodexGenericCredentials(line) + newline)
			continue
		}
		leading := value[:len(value)-len(strings.TrimLeft(value, " \t"))]
		trailing := value[len(strings.TrimRight(value, " \t")):]
		builder.WriteString(line[:separator+1] + leading + `"[REDACTED]"` + trailing + comment + newline)
		trimmedValue := strings.TrimSpace(value)
		if strings.HasPrefix(trimmedValue, `"""`) && !strings.Contains(strings.TrimPrefix(trimmedValue, `"""`), `"""`) {
			maskedMultiline = `"""`
		} else if strings.HasPrefix(trimmedValue, `'''`) && !strings.Contains(strings.TrimPrefix(trimmedValue, `'''`), `'''`) {
			maskedMultiline = `'''`
		} else if bracketDepth := codexBracketDelta(trimmedValue); bracketDepth > 0 {
			maskedArrayDepth = bracketDepth
		}
	}
	return redactCodexGenericCredentials(builder.String()), true, nil
}

func limitCodexDiagnosticContent(content string) (string, bool) {
	data := []byte(content)
	if len(data) <= maxCodexDiagnosticFileSize {
		return content, false
	}
	data = data[:maxCodexDiagnosticFileSize]
	if lastNewline := bytes.LastIndexByte(data, '\n'); lastNewline >= 0 {
		data = data[:lastNewline+1]
	}
	return string(data), true
}

func safeCodexConfigValues(path string) map[string]string {
	data, err := readLimitedFile(path, maxCodexSkillFileSize)
	if err != nil {
		return nil
	}
	values := make(map[string]string)
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if section != "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		switch key {
		case "model", "model_provider", "model_reasoning_effort", "sandbox_mode", "approval_policy":
		default:
			continue
		}
		value := strings.TrimSpace(strings.SplitN(parts[1], "#", 2)[0])
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		}
		if value = safeCodexDisplayValue(value); value != "" {
			values[key] = value
		}
	}
	return values
}

func projectRootForCodex(projectDir string) string {
	current := filepath.Clean(projectDir)
	for current != "." && current != string(filepath.Separator) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return filepath.Clean(projectDir)
}

func selectedCodexInstruction(dir string) string {
	override := filepath.Join(dir, "AGENTS.override.md")
	if _, err := os.Stat(override); err == nil {
		return override
	}
	return filepath.Join(dir, "AGENTS.md")
}

func codexInstructionCandidates(projectDir string) []codexPathCandidate {
	var candidates []codexPathCandidate
	seen := make(map[string]bool)
	if home := codexHomeDir(); home != "" {
		addCodexPathCandidate(&candidates, seen, "global", selectedCodexInstruction(home))
	}
	root := projectRootForCodex(projectDir)
	dirs := make([]string, 0, 8)
	for current := filepath.Clean(projectDir); ; current = filepath.Dir(current) {
		dirs = append(dirs, current)
		if current == root || current == filepath.Dir(current) {
			break
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		addCodexPathCandidate(&candidates, seen, "project", selectedCodexInstruction(dirs[i]))
	}
	return candidates
}

func inspectCodexInstruction(candidate codexPathCandidate) api.CodexInstructionFile {
	result := api.CodexInstructionFile{Scope: candidate.scope, Path: candidate.path, Status: "missing"}
	info, err := os.Stat(candidate.path)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Status = "unavailable"
		}
		return result
	}
	result.Exists = !info.IsDir()
	if result.Exists {
		file, openErr := os.Open(candidate.path)
		if openErr != nil {
			result.Status = "unreadable"
			return result
		}
		_ = file.Close()
		result.IsSymlink = codexPathIsSymlink(candidate.path)
		result.Status = "present"
	} else {
		result.Status = "invalid"
	}
	return result
}

func codexSkillRoots(projectDir string) []codexPathCandidate {
	var roots []codexPathCandidate
	seen := make(map[string]bool)
	root := projectRootForCodex(projectDir)
	addCodexPathCandidate(&roots, seen, "project", filepath.Join(root, ".agents", "skills"))
	addCodexPathCandidate(&roots, seen, "project", filepath.Join(root, ".codex", "skills"))
	if home := codexHomeDir(); home != "" {
		addCodexPathCandidate(&roots, seen, "user", filepath.Join(home, "skills"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		addCodexPathCandidate(&roots, seen, "user", filepath.Join(home, ".agents", "skills"))
	}
	addCodexPathCandidate(&roots, seen, "admin", "/etc/codex/skills")
	return roots
}

func parseCodexSkillMetadata(path, fallbackName string) (string, string, string) {
	data, err := readLimitedFile(path, maxCodexDiagnosticFileSize)
	if err != nil {
		return fallbackName, "", "unreadable"
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fallbackName, "", "invalid"
	}
	name, description := fallbackName, ""
	closed := false
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			closed = true
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch strings.TrimSpace(parts[0]) {
		case "name":
			name = safeCodexDisplayValue(value)
		case "description":
			description = safeCodexDisplayValue(value)
		}
	}
	if !closed || name == "" {
		return name, description, "invalid"
	}
	return name, description, "available"
}

func readCodexSkillNames(path string) ([]string, bool, error) {
	directory, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, false, nil
		}
		return nil, false, errors.New("skills directory could not be read")
	}
	defer func() { _ = directory.Close() }()
	names, err := directory.Readdirnames(maxCodexSkills + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	truncated := len(names) > maxCodexSkills
	if truncated {
		names = names[:maxCodexSkills]
	}
	return names, truncated, nil
}

func scanCodexSkills(root codexPathCandidate) ([]api.CodexSkill, bool, error) {
	names, truncated, err := readCodexSkillNames(root.path)
	if err != nil {
		return nil, false, err
	}
	result := make([]api.CodexSkill, 0)
	for _, name := range names {
		path := filepath.Join(root.path, name, "SKILL.md")
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		displayName, description, status := parseCodexSkillMetadata(path, name)
		result = append(result, api.CodexSkill{
			Name:        displayName,
			Description: description,
			Scope:       root.scope,
			Path:        path,
			Status:      status,
			IsSymlink:   codexPathIsSymlink(path),
		})
	}
	return result, truncated, nil
}

func appendCodexError(diagnostics *api.CodexDiagnostics, section, message string) {
	diagnostics.Errors = append(diagnostics.Errors, api.CodexDiagnosticError{Section: section, Message: message})
}

func (s *Server) GetCodexDiagnostics(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}

	projectDir := filepath.Clean(app.Project.Path)
	diagnostics := &api.CodexDiagnostics{
		Project:      app.Project.Name,
		FetchedAt:    time.Now().UTC(),
		ConfigFiles:  []api.CodexConfigFile{},
		Instructions: []api.CodexInstructionFile{},
		Skills:       []api.CodexSkill{},
		McpServers:   []api.CodexMCPServer{},
		Errors:       []api.CodexDiagnosticError{},
		Codex: api.CodexRuntime{
			Status:        "unavailable",
			FeatureFlags:  []string{},
			StatusReasons: []string{},
		},
	}
	skillsTruncated := false

	ctx, cancel := context.WithTimeout(r.Context(), codexDiagnosticsTimeout)
	defer cancel()
	doctorOut, doctorErr := s.runCodex(ctx, projectDir, "doctor", "--json")
	if doctorErr != nil {
		appendCodexError(diagnostics, "codex", "Codex doctor could not be executed")
	} else if runtime, parseErr := codexRuntimeFromDoctor(doctorOut); parseErr != nil {
		appendCodexError(diagnostics, "codex", "Codex doctor returned an invalid report")
	} else {
		diagnostics.Codex = runtime
	}
	if executable, lookErr := exec.LookPath("codex"); lookErr == nil {
		diagnostics.Codex.ExecutablePath = &executable
	}

	configCandidates := codexConfigCandidates(projectDir)
	loadedPath := codexDetailFromDoctor(doctorOut, "config.load", "config.toml")
	configParseStatus := codexDetailFromDoctor(doctorOut, "config.load", "config.toml parse")
	configValues := make(map[string]string)
	for _, candidate := range configCandidates {
		config := inspectCodexConfigFile(candidate, loadedPath, configParseStatus)
		diagnostics.ConfigFiles = append(diagnostics.ConfigFiles, config)
		if config.Readable {
			for key, value := range safeCodexConfigValues(candidate.path) {
				configValues[key] = value
			}
		}
	}
	if diagnostics.Codex.ReasoningEffort == nil {
		setCodexString(&diagnostics.Codex.ReasoningEffort, configValues["model_reasoning_effort"])
	}
	if diagnostics.Codex.SandboxMode == nil {
		setCodexString(&diagnostics.Codex.SandboxMode, configValues["sandbox_mode"])
	}
	if diagnostics.Codex.ApprovalPolicy == nil {
		setCodexString(&diagnostics.Codex.ApprovalPolicy, configValues["approval_policy"])
	}

	for _, candidate := range codexInstructionCandidates(projectDir) {
		diagnostics.Instructions = append(diagnostics.Instructions, inspectCodexInstruction(candidate))
	}
	for _, root := range codexSkillRoots(projectDir) {
		skills, truncated, scanErr := scanCodexSkills(root)
		if scanErr != nil {
			appendCodexError(diagnostics, "skills", "A skills directory could not be read")
			continue
		}
		diagnostics.Skills = append(diagnostics.Skills, skills...)
		skillsTruncated = skillsTruncated || truncated
	}
	diagnostics.SkillsTruncated = skillsTruncated
	sort.Slice(diagnostics.Skills, func(i, j int) bool {
		return diagnostics.Skills[i].Path < diagnostics.Skills[j].Path
	})

	featuresOut, featuresErr := s.runCodex(ctx, projectDir, "features", "list")
	if featuresErr != nil {
		appendCodexError(diagnostics, "features", "Codex feature flags could not be read")
	} else if features := parseCodexFeatureList(featuresOut); len(features) > 0 {
		diagnostics.Codex.FeatureFlags = features
	}

	mcpOut, mcpErr := s.runCodex(ctx, projectDir, "mcp", "list")
	if mcpErr != nil {
		appendCodexError(diagnostics, "mcp", "MCP configuration could not be read")
	} else {
		diagnostics.McpServers = parseCodexMCPList(mcpOut)
	}

	writeJSONResponse(w, http.StatusOK, diagnostics)
}

func (s *Server) GetCodexDiagnosticContent(w http.ResponseWriter, r *http.Request, params api.GetCodexDiagnosticContentParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if params.Kind != api.GetCodexDiagnosticContentParamsKindConfig &&
		params.Kind != api.GetCodexDiagnosticContentParamsKindInstruction &&
		params.Kind != api.GetCodexDiagnosticContentParamsKindSkill {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("unsupported diagnostic content kind")})
		return
	}

	candidate, ok := findCodexContentCandidate(app.Project.Path, params.Kind, params.Path)
	if !ok {
		writeError(w, &httpError{status: http.StatusNotFound, err: errors.New("diagnostic content is not available")})
		return
	}
	data, size, truncated, err := readCodexDiagnosticFile(candidate.path)
	if err != nil {
		writeError(w, &httpError{status: http.StatusNotFound, err: errors.New("diagnostic content is not readable")})
		return
	}

	content := string(data)
	redacted := false
	if params.Kind == api.GetCodexDiagnosticContentParamsKindConfig {
		content, redacted, err = redactCodexConfigContent(data)
		if err != nil {
			writeError(w, &httpError{status: http.StatusNotFound, err: errors.New("configuration content could not be safely displayed")})
			return
		}
	}
	if limitedContent, contentTruncated := limitCodexDiagnosticContent(content); contentTruncated {
		content = limitedContent
		truncated = true
	}

	writeJSONResponse(w, http.StatusOK, api.CodexDiagnosticContent{
		Kind:      api.CodexDiagnosticContentKind(params.Kind),
		Scope:     candidate.scope,
		Path:      candidate.path,
		Content:   content,
		IsSymlink: candidate.isSymlink,
		Redacted:  redacted,
		Truncated: truncated,
		SizeBytes: size,
		FetchedAt: time.Now().UTC(),
	})
}

func (s *Server) GetCodexSkillFiles(w http.ResponseWriter, r *http.Request, params api.GetCodexSkillFilesParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}

	candidate, ok := findCodexSkillDirectoryCandidate(app.Project.Path, params.Path)
	if !ok {
		writeError(w, &httpError{status: http.StatusNotFound, err: errors.New("diagnostic skill directory is not available")})
		return
	}
	entries, truncated, err := readCodexSkillDirectory(candidate.path)
	if err != nil {
		writeError(w, &httpError{status: http.StatusNotFound, err: errors.New("diagnostic skill directory is not readable")})
		return
	}

	writeJSONResponse(w, http.StatusOK, api.CodexSkillFileList{
		Path:      candidate.path,
		Entries:   entries,
		Truncated: truncated,
		FetchedAt: time.Now().UTC(),
	})
}

func codexDetailFromDoctor(out []byte, checkID, key string) string {
	if len(out) == 0 {
		return ""
	}
	var report rawCodexDoctor
	if json.Unmarshal(out, &report) != nil {
		return ""
	}
	return codexDetail(report, checkID, key)
}
