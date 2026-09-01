package markdown

import (
	"context"
	"sort"
	"strings"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type Searcher struct {
	root string
}

func NewSearcher(root string) *Searcher {
	return &Searcher{root: root}
}

type scoredResult struct {
	result domain.SearchResult
	score  int
}

func (s *Searcher) Search(ctx context.Context, query string, option domain.SearchOption) ([]domain.SearchResult, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}
	var scored []scoredResult
	if option.Type == "" || option.Type == domain.SearchTypeKnowledge {
		kr := NewKnowledgeRepository(s.root)
		knowledge, err := kr.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, k := range knowledge {
			if sr, score, ok := matchKnowledge(k, q); ok {
				scored = append(scored, scoredResult{result: sr, score: score})
			}
		}
	}
	if option.Type == "" || option.Type == domain.SearchTypeTask {
		tr := NewTaskRepository(s.root)
		tasks, err := tr.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range tasks {
			if sr, score, ok := matchTask(t, q); ok {
				scored = append(scored, scoredResult{result: sr, score: score})
			}
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].result.Title < scored[j].result.Title
	})
	results := make([]domain.SearchResult, 0, len(scored))
	for _, sr := range scored {
		results = append(results, sr.result)
	}
	return results, nil
}

func matchTask(t domain.Task, q string) (domain.SearchResult, int, bool) {
	score := 0
	if strings.Contains(strings.ToLower(t.Title), q) {
		score += 10
	}
	if strings.Contains(strings.ToLower(t.ID), q) {
		score += 5
	}
	if strings.Contains(strings.ToLower(t.Assignee), q) {
		score += 3
	}
	if strings.Contains(strings.ToLower(string(t.Status)), q) {
		score += 3
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			score += 3
		}
	}
	body := strings.ToLower(t.Body)
	idx := strings.Index(body, q)
	if idx >= 0 {
		score++
	}
	if score == 0 {
		return domain.SearchResult{}, 0, false
	}
	path := "tasks/" + t.ID + "/task.md"
	if t.Type == domain.TaskTypeAdhoc {
		path = "tasks/adhoc/" + t.ID + ".md"
	}
	return domain.SearchResult{
		Type:    domain.SearchTypeTask,
		ID:      t.ID,
		Path:    path,
		Title:   t.Title,
		Snippet: snippet(t.Body, idx),
	}, score, true
}

func matchKnowledge(k domain.Knowledge, q string) (domain.SearchResult, int, bool) {
	score := 0
	if strings.Contains(strings.ToLower(k.Title), q) {
		score += 10
	}
	if strings.Contains(strings.ToLower(k.Path), q) {
		score += 5
	}
	for _, alias := range k.Aliases {
		if strings.Contains(strings.ToLower(alias), q) {
			score += 5
		}
	}
	for _, tag := range k.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			score += 3
		}
	}
	body := strings.ToLower(k.Body)
	idx := strings.Index(body, q)
	if idx >= 0 {
		score++
	}
	if score == 0 {
		return domain.SearchResult{}, 0, false
	}
	return domain.SearchResult{
		Type:    domain.SearchTypeKnowledge,
		Path:    k.Path,
		Title:   k.Title,
		Snippet: snippet(k.Body, idx),
	}, score, true
}

func snippet(text string, idx int) string {
	if text == "" {
		return ""
	}
	start := idx - 30
	if start < 0 {
		start = 0
	}
	end := idx + 80
	if idx < 0 {
		end = 80
	}
	if end > len(text) {
		end = len(text)
	}
	body := strings.ReplaceAll(text[start:end], "\n", " ")
	return "..." + strings.TrimSpace(body) + "..."
}
