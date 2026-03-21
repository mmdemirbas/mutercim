package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

// FailoverChain implements Provider by trying multiple providers in order.
// When a provider returns a quota error (HTTP 429), it is marked as exhausted
// and the next provider is tried. Exhausted providers recover after a configurable window.
type FailoverChain struct {
	entries        []chainEntry
	recoveryWindow time.Duration
	mu             sync.Mutex
	logger         *slog.Logger
	now            func() time.Time // injectable for testing
	// OnFailover is called when switching to a different provider due to quota exhaustion.
	OnFailover func(exhaustedProvider, nextProvider string)
}

type chainEntry struct {
	provider       Provider
	client         *apiclient.Client // for cleanup
	exhaustedUntil time.Time
}

// NewFailoverChain creates a failover chain from the given providers.
// Each provider should have its own apiclient.Client with its own rate limiter.
// recoveryWindow is the duration before an exhausted provider becomes eligible again.
func NewFailoverChain(providers []Provider, clients []*apiclient.Client, recoveryWindow time.Duration, logger *slog.Logger) *FailoverChain {
	if logger == nil {
		logger = slog.Default()
	}
	entries := make([]chainEntry, len(providers))
	for i, p := range providers {
		var c *apiclient.Client
		if i < len(clients) {
			c = clients[i]
		}
		entries[i] = chainEntry{provider: p, client: c}
	}
	return &FailoverChain{
		entries:        entries,
		recoveryWindow: recoveryWindow,
		logger:         logger,
		now:            time.Now,
	}
}

// Name returns a composite name of all providers in the chain.
func (f *FailoverChain) Name() string {
	names := make([]string, len(f.entries))
	for i, e := range f.entries {
		names[i] = e.provider.Name()
	}
	return "failover(" + strings.Join(names, ",") + ")"
}

// SupportsVision returns true if any provider in the chain supports vision.
func (f *FailoverChain) SupportsVision() bool {
	for _, e := range f.entries {
		if e.provider.SupportsVision() {
			return true
		}
	}
	return false
}

// ReadFromImage tries each vision-capable provider in order, failing over on quota errors.
func (f *FailoverChain) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	return f.tryProviders(ctx, true, func(p Provider) (string, error) {
		return p.ReadFromImage(ctx, image, systemPrompt, userPrompt)
	})
}

// Translate tries each provider in order, failing over on quota errors.
func (f *FailoverChain) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return f.tryProviders(ctx, false, func(p Provider) (string, error) {
		return p.Translate(ctx, systemPrompt, userPrompt)
	})
}

// SetRetryCallback sets the OnRetry callback on all clients in the chain.
func (f *FailoverChain) SetRetryCallback(fn func(attempt, maxRetries, statusCode int, backoff time.Duration)) {
	for i := range f.entries {
		if f.entries[i].client != nil {
			f.entries[i].client.OnRetry = fn
		}
	}
}

// Close releases resources held by all providers' clients.
func (f *FailoverChain) Close() {
	for _, e := range f.entries {
		if e.client != nil {
			e.client.Close()
		}
	}
}

// ActiveProvider returns the name of the first non-exhausted, eligible provider.
func (f *FailoverChain) ActiveProvider(needsVision bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	for _, e := range f.entries {
		if needsVision && !e.provider.SupportsVision() {
			continue
		}
		if now.Before(e.exhaustedUntil) {
			continue
		}
		return e.provider.Name()
	}
	return ""
}

func (f *FailoverChain) tryProviders(ctx context.Context, needsVision bool, fn func(Provider) (string, error)) (string, error) {
	f.mu.Lock()
	now := f.now()
	f.mu.Unlock()

	var errs []error
	for i := range f.entries {
		e := &f.entries[i]
		p := e.provider

		if needsVision && !p.SupportsVision() {
			f.logger.Info("skipping provider for read (no vision support)", "provider", p.Name())
			continue
		}

		f.mu.Lock()
		exhausted := now.Before(e.exhaustedUntil)
		f.mu.Unlock()
		if exhausted {
			continue
		}

		result, err := fn(p)
		if err == nil {
			return result, nil
		}

		if isQuotaError(err) {
			f.mu.Lock()
			e.exhaustedUntil = f.now().Add(f.recoveryWindow)
			f.mu.Unlock()

			// Find next eligible provider name for the callback
			nextName := ""
			for j := i + 1; j < len(f.entries); j++ {
				ne := &f.entries[j]
				if needsVision && !ne.provider.SupportsVision() {
					continue
				}
				f.mu.Lock()
				nexhausted := now.Before(ne.exhaustedUntil)
				f.mu.Unlock()
				if !nexhausted {
					nextName = ne.provider.Name()
					break
				}
			}

			f.logger.Warn("provider exhausted (429), failing over to next",
				"provider", p.Name(),
				"next", nextName,
				"recovery_seconds", f.recoveryWindow.Seconds(),
			)
			if f.OnFailover != nil && nextName != "" {
				f.OnFailover(p.Name(), nextName)
			}
			errs = append(errs, fmt.Errorf("%s: %w", p.Name(), err))
			continue
		}

		// Non-quota errors are not retried on next provider
		return "", err
	}

	if len(errs) > 0 {
		return "", fmt.Errorf("all providers exhausted: %w", errors.Join(errs...))
	}
	return "", fmt.Errorf("no eligible providers in failover chain")
}

func isQuotaError(err error) bool {
	var httpErr *apiclient.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 429
	}
	// Also check wrapped "max retries exceeded" errors
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		return isQuotaError(unwrapped)
	}
	return false
}
