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
	SilencedBy  []string
	InhibitedBy []string
}

type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

type Silence struct {
	ID        string    `json:"id,omitempty"`
	Status    string    `json:"status,omitempty"`
	Matchers  []Matcher `json:"matchers"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
}

type AlertRepository interface {
	GetAlerts(ctx context.Context, filters ...string) ([]Alert, error)
}

type SilenceRepository interface {
	List(ctx context.Context, filters ...string) ([]Silence, error)
	Create(ctx context.Context, silence Silence) (string, error)
	Delete(ctx context.Context, id string) error
}
