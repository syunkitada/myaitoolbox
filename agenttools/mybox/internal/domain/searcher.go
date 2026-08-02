package domain

import "context"

type SearchType string

const (
	SearchTypeTask      SearchType = "task"
	SearchTypeKnowledge SearchType = "knowledge"
)

type SearchOption struct {
	Type SearchType
}

type SearchResult struct {
	Type    SearchType
	ID      string
	Path    string
	Title   string
	Snippet string
}

type Searcher interface {
	Search(ctx context.Context, query string, option SearchOption) ([]SearchResult, error)
}
