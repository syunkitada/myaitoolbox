package entrypoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type schedulerTestStore struct {
	prompts []domain.ScheduledPrompt
	deleted []string
}

func (s *schedulerTestStore) List(context.Context, string) ([]domain.ScheduledPrompt, error) {
	return append([]domain.ScheduledPrompt(nil), s.prompts...), nil
}

func (*schedulerTestStore) Create(context.Context, domain.ScheduledPrompt) error { return nil }

func (s *schedulerTestStore) Delete(_ context.Context, _, id string) error {
	s.deleted = append(s.deleted, id)
	for i, prompt := range s.prompts {
		if prompt.ID == id {
			s.prompts = append(s.prompts[:i], s.prompts[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func TestPromptSchedulerSendsDuePromptsAndKeepsFuturePrompts(t *testing.T) {
	now := time.Unix(1000, 0)
	store := &schedulerTestStore{prompts: []domain.ScheduledPrompt{
		{ID: "due", Project: "test", Target: "agent", Text: "send now", ScheduledAt: now.Add(-time.Second)},
		{ID: "future", Project: "test", Target: "agent", Text: "send later", ScheduledAt: now.Add(time.Minute)},
	}}
	var calls [][]string
	scheduler := newPromptScheduler(store, func(_ context.Context, prompt domain.ScheduledPrompt) error {
		calls = append(calls, []string{"agent", "prompt", prompt.Target, prompt.Text})
		return nil
	})

	scheduler.processDue(context.Background(), now)

	require.Equal(t, [][]string{{"agent", "prompt", "agent", "send now"}}, calls)
	assert.Equal(t, []string{"due"}, store.deleted)
	assert.Len(t, store.prompts, 1)
	assert.Equal(t, "future", store.prompts[0].ID)
}

func TestPromptSchedulerRetriesFailedPromptLater(t *testing.T) {
	now := time.Unix(1000, 0)
	store := &schedulerTestStore{prompts: []domain.ScheduledPrompt{{
		ID: "retry", Project: "test", Target: "agent", Text: "try", ScheduledAt: now,
	}}}
	calls := 0
	scheduler := newPromptScheduler(store, func(context.Context, domain.ScheduledPrompt) error {
		calls++
		return errors.New("agent unavailable")
	})

	scheduler.processDue(context.Background(), now)
	scheduler.processDue(context.Background(), now.Add(time.Second))

	assert.Equal(t, 1, calls)
	assert.Empty(t, store.deleted)
	assert.Equal(t, now.Add(scheduledPromptRetryInterval), scheduler.retryAt["retry"])
}
