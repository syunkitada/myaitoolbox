package automation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/robfig/cron/v3"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

const definitionsDir = "_task_triggers"

type DefinitionRepository struct {
	projectRoot string
}

func NewDefinitionRepository(projectRoot string) *DefinitionRepository {
	return &DefinitionRepository{projectRoot: projectRoot}
}

type rawDefinition struct {
	Version   int          `yaml:"version"`
	Enabled   bool         `yaml:"enabled"`
	Trigger   rawTrigger   `yaml:"trigger"`
	Task      rawTask      `yaml:"task"`
	Execution rawExecution `yaml:"execution"`
}

type rawTrigger struct {
	Type      string `yaml:"type"`
	Cron      string `yaml:"cron"`
	Timezone  string `yaml:"timezone"`
	Path      string `yaml:"path"`
	Pattern   string `yaml:"pattern"`
	SettleFor string `yaml:"settle_for"`
}

type rawTask struct {
	Template  string   `yaml:"template"`
	AgentKind string   `yaml:"agent_kind"`
	Prompt    string   `yaml:"prompt"`
	Tags      []string `yaml:"tags"`
}

type rawExecution struct {
	Overlap string `yaml:"overlap"`
	Misfire string `yaml:"misfire"`
}

type definitionFile struct {
	Version int         `yaml:"version"`
	Enabled bool        `yaml:"enabled"`
	Trigger triggerFile `yaml:"trigger"`
	Task    taskFile    `yaml:"task"`
}

type triggerFile struct {
	Type     string `yaml:"type"`
	Cron     string `yaml:"cron,omitempty"`
	Timezone string `yaml:"timezone,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Pattern  string `yaml:"pattern,omitempty"`
}

type taskFile struct {
	Template  string `yaml:"template"`
	AgentKind string `yaml:"agent_kind"`
	Prompt    string `yaml:"prompt,omitempty"`
}

// Create validates and persists one complete trigger definition without
// replacing an existing definition. The temporary directory is validated by
// the same loader used by List before it is moved into the project tree.
func (r *DefinitionRepository) Create(ctx context.Context, input domain.TriggerDefinitionInput, taskContent string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validID(input.ID) {
		return fmt.Errorf("%w: invalid trigger id %q", domain.ErrInvalidArgument, input.ID)
	}
	if input.Enabled && strings.TrimSpace(input.AgentKind) == "" {
		return fmt.Errorf("%w: task.agent_kind is required for enabled triggers", domain.ErrInvalidArgument)
	}

	root := filepath.Join(r.projectRoot, definitionsDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	finalDir := filepath.Join(root, input.ID)
	if _, err := os.Lstat(finalDir); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, input.ID)
	} else if !os.IsNotExist(err) {
		return err
	}

	tmpDir, err := os.MkdirTemp(root, "."+input.ID+".tmp-")
	if err != nil {
		return err
	}
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	pattern := strings.TrimSpace(input.Pattern)
	if input.Type == domain.TriggerTypeFileCreated && pattern == "" {
		pattern = "*"
	}
	definition := definitionFile{
		Version: 1,
		Enabled: input.Enabled,
		Trigger: triggerFile{
			Type:     string(input.Type),
			Cron:     strings.TrimSpace(input.Cron),
			Timezone: strings.TrimSpace(input.Timezone),
			Path:     strings.TrimSpace(input.WatchPath),
			Pattern:  pattern,
		},
		Task: taskFile{
			Template:  "task.md",
			AgentKind: strings.TrimSpace(input.AgentKind),
			Prompt:    strings.TrimSpace(input.Prompt),
		},
	}
	data, err := yaml.Marshal(definition)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(tmpDir, "trigger.yaml"), data, 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(tmpDir, "task.md"), []byte(taskContent), 0o644); err != nil {
		return err
	}
	if input.Type == domain.TriggerTypeFileCreated {
		watchPath, err := relativePath(tmpDir, input.WatchPath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(watchPath, 0o755); err != nil {
			return err
		}
	}
	if _, err := r.load(tmpDir, input.ID); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(finalDir); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, input.ID)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, input.ID)
		}
		return err
	}
	keepTemp = true
	return nil
}

func (r *DefinitionRepository) List(ctx context.Context) ([]domain.TriggerDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := filepath.Join(r.projectRoot, definitionsDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.TriggerDefinition{}, nil
		}
		return nil, err
	}

	defs := make([]domain.TriggerDefinition, 0)
	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		def, err := r.load(filepath.Join(root, entry.Name()), entry.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		defs = append(defs, *def)
	}
	return defs, errors.Join(errs...)
}

func (r *DefinitionRepository) Find(ctx context.Context, id string) (*domain.TriggerDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, fmt.Errorf("%w: invalid trigger id %q", domain.ErrInvalidArgument, id)
	}
	return r.load(filepath.Join(r.projectRoot, definitionsDir, id), id)
}

func (r *DefinitionRepository) load(dir, id string) (*domain.TriggerDefinition, error) {
	if !validID(id) {
		return nil, fmt.Errorf("%w: invalid trigger id %q", domain.ErrInvalidArgument, id)
	}
	configPath := filepath.Join(dir, "trigger.yaml")
	if err := regularFile(configPath); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var raw rawDefinition
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}
	if raw.Version != 1 {
		return nil, fmt.Errorf("%s: %w: unsupported version %d", configPath, domain.ErrInvalidArgument, raw.Version)
	}
	if raw.Trigger.Type != string(domain.TriggerTypeCron) &&
		raw.Trigger.Type != string(domain.TriggerTypeFileCreated) &&
		raw.Trigger.Type != string(domain.TriggerTypeManual) {
		return nil, fmt.Errorf("%s: %w: unsupported trigger type %q", configPath, domain.ErrInvalidArgument, raw.Trigger.Type)
	}

	template := raw.Task.Template
	if template == "" {
		template = "task.md"
	}
	templatePath, err := relativePath(dir, template)
	if err != nil {
		return nil, err
	}
	if err := regularFile(templatePath); err != nil {
		return nil, err
	}

	def := &domain.TriggerDefinition{
		ID:           id,
		ProjectRoot:  r.projectRoot,
		RootPath:     dir,
		Enabled:      raw.Enabled,
		Type:         domain.TriggerType(raw.Trigger.Type),
		Cron:         strings.TrimSpace(raw.Trigger.Cron),
		Timezone:     strings.TrimSpace(raw.Trigger.Timezone),
		TemplatePath: templatePath,
		AgentKind:    strings.TrimSpace(raw.Task.AgentKind),
		Prompt:       strings.TrimSpace(raw.Task.Prompt),
		Tags:         append([]string(nil), raw.Task.Tags...),
		Overlap:      domain.OverlapPolicy(raw.Execution.Overlap),
		Misfire:      domain.MisfirePolicy(raw.Execution.Misfire),
	}
	if def.Overlap == "" {
		if def.Type == domain.TriggerTypeFileCreated || def.Type == domain.TriggerTypeManual {
			def.Overlap = domain.OverlapAllow
		} else {
			def.Overlap = domain.OverlapSkip
		}
	}
	if def.Misfire == "" {
		def.Misfire = domain.MisfireSkip
	}
	if def.Pattern == "" {
		def.Pattern = "*"
	}
	if raw.Trigger.SettleFor == "" {
		def.SettleFor = 2 * time.Second
	} else {
		def.SettleFor, err = time.ParseDuration(raw.Trigger.SettleFor)
		if err != nil || def.SettleFor < 0 {
			return nil, fmt.Errorf("%s: %w: invalid settle_for %q", configPath, domain.ErrInvalidArgument, raw.Trigger.SettleFor)
		}
	}

	switch def.Type {
	case domain.TriggerTypeCron:
		if def.Cron == "" || def.Timezone == "" {
			return nil, fmt.Errorf("%s: %w: cron and timezone are required", configPath, domain.ErrInvalidArgument)
		}
		if _, err := time.LoadLocation(def.Timezone); err != nil {
			return nil, fmt.Errorf("%s: %w: invalid timezone %q", configPath, domain.ErrInvalidArgument, def.Timezone)
		}
		if _, err := cron.ParseStandard(def.Cron); err != nil {
			return nil, fmt.Errorf("%s: %w: invalid cron %q: %v", configPath, domain.ErrInvalidArgument, def.Cron, err)
		}
	case domain.TriggerTypeFileCreated:
		if raw.Trigger.Path == "" {
			return nil, fmt.Errorf("%s: %w: trigger.path is required", configPath, domain.ErrInvalidArgument)
		}
		watchPath, err := relativePath(dir, raw.Trigger.Path)
		if err != nil {
			return nil, err
		}
		def.WatchPath = watchPath
		def.Pattern = raw.Trigger.Pattern
		if def.Pattern == "" {
			def.Pattern = "*"
		}
		if _, err := filepath.Match(def.Pattern, "example"); err != nil {
			return nil, fmt.Errorf("%s: %w: invalid pattern %q", configPath, domain.ErrInvalidArgument, def.Pattern)
		}
	}
	if def.Overlap != domain.OverlapSkip && def.Overlap != domain.OverlapAllow {
		return nil, fmt.Errorf("%s: %w: invalid overlap %q", configPath, domain.ErrInvalidArgument, def.Overlap)
	}
	if def.Misfire != domain.MisfireSkip {
		return nil, fmt.Errorf("%s: %w: invalid misfire %q", configPath, domain.ErrInvalidArgument, def.Misfire)
	}
	if def.Enabled && strings.TrimSpace(def.AgentKind) == "" {
		return nil, fmt.Errorf("%s: %w: task.agent_kind is required for enabled triggers", configPath, domain.ErrInvalidArgument)
	}
	return def, nil
}

func regularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", domain.ErrInvalidPath, path)
	}
	return nil
}

func relativePath(root, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || filepath.IsAbs(raw) || strings.ContainsAny(raw, `\`) {
		return "", fmt.Errorf("%w: %q", domain.ErrInvalidPath, raw)
	}
	clean := filepath.Clean(filepath.FromSlash(raw))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", domain.ErrInvalidPath, raw)
	}
	return filepath.Join(root, clean), nil
}

func validID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}
