package domain

import "context"

type Favorite struct {
	Project string
	Path    string
}

type State struct {
	Favorites   []Favorite
	RecentFiles []string
}

type StateStore interface {
	Load(ctx context.Context) (*State, error)
	Save(ctx context.Context, state *State) error
	// Update executes fn atomically on the persisted state, so that
	// concurrent read-modify-write cycles cannot drop changes.
	Update(ctx context.Context, fn func(*State) error) error
}
