package entrypoint

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

const scheduledPromptPollInterval = time.Second
const scheduledPromptRetryInterval = 30 * time.Second

type promptScheduler struct {
	store domain.ScheduledPromptRepository
	run   scheduledPromptRunFunc

	mu      sync.Mutex
	retryAt map[string]time.Time
}

type scheduledPromptRunFunc func(context.Context, domain.ScheduledPrompt) error

func newPromptScheduler(store domain.ScheduledPromptRepository, run scheduledPromptRunFunc) *promptScheduler {
	return &promptScheduler{store: store, run: run, retryAt: make(map[string]time.Time)}
}

// Start runs the scheduler until the server context is cancelled. The first
// pass happens immediately so prompts that became due while mybox was stopped
// are delivered without waiting for the next interval.
func (s *promptScheduler) Start(ctx context.Context) {
	go func() {
		s.processDue(ctx, time.Now())
		ticker := time.NewTicker(scheduledPromptPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				s.processDue(ctx, now)
			}
		}
	}()
}

func (s *promptScheduler) processDue(ctx context.Context, now time.Time) {
	prompts, err := s.store.List(ctx, "")
	if err != nil {
		return
	}
	for _, prompt := range prompts {
		if prompt.ScheduledAt.After(now) || s.isRetryDeferred(prompt.ID, now) {
			continue
		}
		if err := s.run(ctx, prompt); err != nil {
			s.deferRetry(prompt.ID, now.Add(scheduledPromptRetryInterval))
			continue
		}
		if err := s.store.Delete(ctx, prompt.Project, prompt.ID); err == nil || errors.Is(err, domain.ErrNotFound) {
			s.clearRetry(prompt.ID)
		} else {
			s.deferRetry(prompt.ID, now.Add(scheduledPromptRetryInterval))
		}
	}
}

func (s *promptScheduler) isRetryDeferred(id string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	retryAt, ok := s.retryAt[id]
	return ok && retryAt.After(now)
}

func (s *promptScheduler) deferRetry(id string, retryAt time.Time) {
	s.mu.Lock()
	s.retryAt[id] = retryAt
	s.mu.Unlock()
}

func (s *promptScheduler) clearRetry(id string) {
	s.mu.Lock()
	delete(s.retryAt, id)
	s.mu.Unlock()
}
