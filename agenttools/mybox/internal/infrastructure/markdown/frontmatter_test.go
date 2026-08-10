package markdown

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestSplitFrontMatter(t *testing.T) {
	content := "---\ntitle: Hello\ntags:\n  - a\n---\n\nbody"
	fm, body, ok := splitFrontMatter(content)
	require.True(t, ok)
	assert.Contains(t, fm, "title: Hello")
	assert.Equal(t, "body", body)
}

func TestSplitFrontMatterWithoutFrontMatter(t *testing.T) {
	fm, body, ok := splitFrontMatter("# just body")
	assert.False(t, ok)
	assert.Empty(t, fm)
	assert.Equal(t, "# just body", body)
}

func TestParseBuildRoundTrip(t *testing.T) {
	fields := map[string]any{
		"title":  "Hello",
		"tags":   []string{"a", "b"},
		"status": "todo",
	}
	out, err := buildFrontMatter(fields, "# Body\n")
	require.NoError(t, err)
	fm, body, ok := splitFrontMatter(out)
	require.True(t, ok)
	parsed, err := parseFrontMatter(fm)
	require.NoError(t, err)
	assert.Equal(t, "Hello", parsed["title"])
	assert.Equal(t, []any{"a", "b"}, parsed["tags"])
	assert.Contains(t, body, "# Body")
}

func TestParseFrontMatterEmpty(t *testing.T) {
	fm, err := parseFrontMatter("")
	require.NoError(t, err)
	assert.Empty(t, fm)
}

func TestTaskTemplateQuotesYamlValues(t *testing.T) {
	r := NewTemplateRenderer(t.TempDir(), t.TempDir())
	out, err := r.RenderTask(domain.TaskTemplateData{
		Name: "mcp: mysql: test",
	})
	require.NoError(t, err)

	fm, body, ok := splitFrontMatter(out)
	require.True(t, ok)
	parsed, err := parseFrontMatter(fm)
	require.NoError(t, err)
	assert.Equal(t, "mcp: mysql: test", parsed["title"])
	assert.Contains(t, fm, `title: "mcp: mysql: test"`)
	assert.Contains(t, body, "# mcp: mysql: test")
	assert.NotContains(t, fm, "id:")
	assert.NotContains(t, fm, "project:")
	assert.NotContains(t, fm, "created")
}

func TestKnowledgeTemplateQuotesYamlValues(t *testing.T) {
	r := NewTemplateRenderer(t.TempDir(), t.TempDir())
	out, err := r.RenderKnowledge(domain.KnowledgeTemplateData{
		Path:    "notes/mcp",
		Name:    `mcp: mysql "connector"`,
		Created: time.Date(2026, 8, 10, 13, 19, 13, 0, time.Local),
	})
	require.NoError(t, err)

	fm, _, ok := splitFrontMatter(out)
	require.True(t, ok)
	parsed, err := parseFrontMatter(fm)
	require.NoError(t, err)
	assert.Equal(t, `mcp: mysql "connector"`, parsed["title"])
}
