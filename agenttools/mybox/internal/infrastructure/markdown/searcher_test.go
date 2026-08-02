package markdown

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func newSearchableRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	taskRepo := NewTaskRepository(root)
	require.NoError(t, taskRepo.Create(context.Background(), "20260801_0900_fix-login",
		"---\ntitle: Fix Login\ntags: [web]\n---\n\noauth flow"))
	require.NoError(t, taskRepo.Create(context.Background(), "20260801_1000_ingress",
		"---\ntitle: Ingress Setup\nstatus: doing\n---\n\nnginx ingress"))

	kRepo := NewKnowledgeRepository(root)
	require.NoError(t, kRepo.Create(context.Background(), "architecture/hexagonal",
		"---\ntitle: Hexagonal\n---\n\nSee [[port]] for details"))
	require.NoError(t, kRepo.Create(context.Background(), "golang/context",
		"---\ntitle: Context\n---\n\nrelated to [[hexagonal]]"))
	return root
}

func TestSearchKnowledge(t *testing.T) {
	s := NewSearcher(newSearchableRepo(t))
	ctx := context.Background()

	results, err := s.Search(ctx, "hexagonal", domain.SearchOption{Type: domain.SearchTypeKnowledge})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, domain.SearchTypeKnowledge, results[0].Type)

	results, err = s.Search(ctx, "hexagonal", domain.SearchOption{Type: domain.SearchTypeTask})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchTaskTitleAndBody(t *testing.T) {
	s := NewSearcher(newSearchableRepo(t))
	ctx := context.Background()

	results, err := s.Search(ctx, "login", domain.SearchOption{Type: domain.SearchTypeTask})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "tasks/20260801_0900_fix-login/task.md", results[0].Path)

	results, err = s.Search(ctx, "nginx", domain.SearchOption{Type: domain.SearchTypeTask})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Ingress Setup", results[0].Title)
}

func TestSearchCrossType(t *testing.T) {
	s := NewSearcher(newSearchableRepo(t))
	results, err := s.Search(context.Background(), "hexagonal", domain.SearchOption{})
	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, domain.SearchTypeKnowledge, r.Type)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	s := NewSearcher(newSearchableRepo(t))
	results, err := s.Search(context.Background(), "  ", domain.SearchOption{})
	require.NoError(t, err)
	assert.Empty(t, results)
}
