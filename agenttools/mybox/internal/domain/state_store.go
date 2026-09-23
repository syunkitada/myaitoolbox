package domain

import "context"

type State struct {
	Favorites   []string
	RecentFiles []string
}

type StateStore interface {
	Load(ctx context.Context) (*State, error)
	Save(ctx context.Context, state *State) error
	// Update executes fn atomically on the persisted state, so that
	// concurrent read-modify-write cycles cannot drop changes.
	Update(ctx context.Context, fn func(*State) error) error
}
