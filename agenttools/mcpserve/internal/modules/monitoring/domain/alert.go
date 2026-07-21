package domain

import (
	"context"
	"time"
)

type AlertStatus string

type Alert struct {
	Labels      map[string]string
	Annotations map[string]string
	Status      AlertStatus
}

type Matcher struct {
	Name    string
	Value   string
	IsRegex bool
	IsEqual bool
}

type Silence struct {
	ID        string
	Matchers  []Matcher
	StartsAt  time.Time
	EndsAt    time.Time
	UpdatedAt time.Time
	CreatedBy string
	Comment   string
}

type AlertRepository interface {
	GetAlerts(ctx context.Context, filters ...string) ([]Alert, error)
}

type SilenceRepository interface {
	List(ctx context.Context, filters ...string) ([]Silence, error)
	Create(ctx context.Context, silence Silence) (string, error)
	Delete(ctx context.Context, id string) error
}
