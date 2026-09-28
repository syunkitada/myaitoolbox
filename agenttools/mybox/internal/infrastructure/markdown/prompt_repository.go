package markdown

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/templates"
)

// PromptRepository loads prompt templates from the project's prompts/
// directory, falling back to the default project's prompts/ directory.
type PromptRepository struct {
	projectRoot string
	defaultRoot string
}

func NewPromptRepository(projectRoot string, defaultRoot string) *PromptRepository {
	return &PromptRepository{projectRoot: projectRoot, defaultRoot: defaultRoot}
}

// Render reads the template prompts/<name>.md and substitutes the variables
// given in vars using the $var syntax (e.g. $task_file_path). An unknown
// variable is left untouched.
func (r *PromptRepository) Render(_ context.Context, name string, vars map[string]string) (string, error) {
	name = strings.TrimSpace(strings.TrimSuffix(name, ".md"))
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("%w: invalid prompt name %q", domain.ErrInvalidArgument, name)
	}
	source, err := r.loadTemplate(name)
	if err != nil {
		return "", err
	}
	return expandPromptVars(source, vars), nil
}

func (r *PromptRepository) loadTemplate(name string) (string, error) {
	rel := "prompts/" + name + ".md"
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, rel),
		filepath.Join(r.projectRoot, rel),
		filepath.Join(r.defaultRoot, rel),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if b, err := os.ReadFile(c); err == nil {
			return string(b), nil
		}
	}
	return builtinPrompt(name)
}

// builtinPrompt loads a bundled prompt template (templates/prompts/), mirroring
// the builtin template fallback used by the task templates.
func builtinPrompt(name string) (string, error) {
	source, err := templates.FS.ReadFile("prompts/" + name + ".yaml")
	if err != nil {
		return "", fmt.Errorf("%w: prompt template %q", domain.ErrNotFound, name)
	}
	return string(source), nil
}

// expandPromptVars replaces $var occurrences in s with the corresponding value
// from vars. A literal "$" is escaped as "$$". Unresolved variables are left as
// they appear in the template.
func expandPromptVars(s string, vars map[string]string) string {
	return domain.ExpandPromptVariables(s, vars)
}
