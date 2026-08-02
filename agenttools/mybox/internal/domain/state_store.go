package domain

import "context"

type State struct {
	Favorites   []string
	RecentFiles []string
}

type StateStore interface {
	Load(ctx context.Context) (*State, error)
	Save(ctx context.Context, state *State) error
}
