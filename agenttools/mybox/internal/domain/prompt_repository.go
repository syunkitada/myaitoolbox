package domain

import "context"

// PromptTemplateRepository renders a prompt template by name. Templates live in
// the project's prompts/ directory (project prompts/prompts, default project
// prompts/prompts, or builtin). Template files are resolved by name with a
// mandatory .md extension and rendered with the given variables (e.g.
// $task_file_path).
type PromptTemplateRepository interface {
	// Render expands the template named name with vars and returns the result.
	// name may omit the .md suffix. It returns ErrNotFound when the template
	// does not exist.
	Render(ctx context.Context, name string, vars map[string]string) (string, error)
}
