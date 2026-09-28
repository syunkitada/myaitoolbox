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

// ExpandPromptVariables replaces $var occurrences in an inline prompt using
// the same syntax as prompt template files. Unknown variables are preserved,
// and $$ escapes a literal dollar sign.
func ExpandPromptVariables(s string, vars map[string]string) string {
	var b []byte
	for i := 0; i < len(s); {
		if s[i] != '$' {
			b = append(b, s[i])
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '$' {
			b = append(b, '$')
			i += 2
			continue
		}
		j := i + 1
		start := j
		for j < len(s) && promptVariableChar(s[j]) {
			j++
		}
		if j == start {
			b = append(b, '$')
			i++
			continue
		}
		key := s[start:j]
		if value, ok := vars[key]; ok {
			b = append(b, value...)
		} else {
			b = append(b, '$')
			b = append(b, key...)
		}
		i = j
	}
	return string(b)
}

func promptVariableChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
