package server

import (
	"context"
	"fmt"
	"sync"
)

// DebounceRegistry tracks active review contexts per pull request and cancels outdated ones.
type DebounceRegistry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewDebounceRegistry creates a new registry.
func NewDebounceRegistry() *DebounceRegistry {
	return &DebounceRegistry{
		cancels: make(map[string]context.CancelFunc),
	}
}

func prKey(providerName, repo, number string) string {
	return fmt.Sprintf("%s:%s:%s", providerName, repo, number)
}

// Start registers a new context for a pull request, cancelling any prior in-flight context.
func (r *DebounceRegistry) Start(ctx context.Context, providerName, repo, number string) (context.Context, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := prKey(providerName, repo, number)
	if oldCancel, exists := r.cancels[key]; exists {
		oldCancel()
	}

	subCtx, cancel := context.WithCancel(ctx)
	r.cancels[key] = cancel

	cleanup := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if currentCancel, ok := r.cancels[key]; ok {
			// Only delete if it hasn't been replaced by a newer cancel func
			_ = currentCancel
			delete(r.cancels, key)
		}
		cancel()
	}

	return subCtx, cleanup
}
