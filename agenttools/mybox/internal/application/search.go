package application

import (
	"context"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type SearchUseCase struct {
	Searcher domain.Searcher
}

func NewSearchUseCase(searcher domain.Searcher) *SearchUseCase {
	return &SearchUseCase{Searcher: searcher}
}

func (u *SearchUseCase) Search(ctx context.Context, query string, option domain.SearchOption) ([]domain.SearchResult, error) {
	return u.Searcher.Search(ctx, query, option)
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
