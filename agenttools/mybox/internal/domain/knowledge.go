package domain

import "time"

type Knowledge struct {
	Path      string
	Title     string
	Aliases   []string
	Tags      []string
	Type      string
	Created   time.Time
	LastMod   time.Time
	WikiLinks []string
	Body      string
}
