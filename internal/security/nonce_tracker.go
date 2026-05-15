package security

import (
	"sync"
	"time"

	"reporting_app/internal/core"
)

// nonceEntry tracks a single nonce's usage state.
type nonceEntry struct {
	firstUse time.Time // when the nonce was first added
	lastUse  time.Time // when the nonce was last used
	usesLeft int       // remaining uses (decremented on each use)
}

// NonceTracker tracks used nonces to prevent replay attacks.
type NonceTracker struct {
	usedNonces map[string]*nonceEntry
	mu         sync.RWMutex
	config     core.NonceConfig
	stopChan   chan struct{}
}

// NewNonceTracker creates a new nonce tracker with the given config.
func NewNonceTracker(cfg core.NonceConfig) *NonceTracker {
	tracker := &NonceTracker{
		usedNonces: make(map[string]*nonceEntry),
		config:     cfg,
		stopChan:   make(chan struct{}),
	}

	go tracker.cleanupLoop()
	return tracker
}

// Add adds a nonce to the tracker. Returns false if the nonce is already tracked.
func (t *NonceTracker) Add(nonce string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.usedNonces[nonce]; exists {
		return false
	}

	t.usedNonces[nonce] = &nonceEntry{
		firstUse: time.Now(),
		lastUse:  time.Now(),
		usesLeft: t.config.MaxUses,
	}
	return true
}

// CheckAndAdd checks if a nonce can be used, and consumes one use if so.
// Returns true if the nonce was valid and one use was consumed.
func (t *NonceTracker) CheckAndAdd(nonce string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, exists := t.usedNonces[nonce]
	if !exists {
		// First use — add to tracker
		t.usedNonces[nonce] = &nonceEntry{
			firstUse: time.Now(),
			lastUse:  time.Now(),
			usesLeft: t.config.MaxUses,
		}
		// Consume first use
		t.usedNonces[nonce].usesLeft--
		return true
	}

	// Check max age
	if time.Since(entry.firstUse) > t.config.MaxAge {
		delete(t.usedNonces, nonce)
		return false
	}

	// Check use window for multi-use nonces
	if t.config.MaxUses > 1 && time.Since(entry.lastUse) > t.config.UseWindow {
		delete(t.usedNonces, nonce)
		return false
	}

	// Check remaining uses
	if entry.usesLeft <= 0 {
		delete(t.usedNonces, nonce)
		return false
	}

	// Consume one use
	entry.usesLeft--
	entry.lastUse = time.Now()
	return true
}

// cleanupLoop periodically removes old nonces.
func (t *NonceTracker) cleanupLoop() {
	ticker := time.NewTicker(t.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.cleanup()
		case <-t.stopChan:
			return
		}
	}
}

// cleanup removes nonces older than MaxAge.
func (t *NonceTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-t.config.MaxAge)
	for nonce, entry := range t.usedNonces {
		if entry.firstUse.Before(cutoff) {
			delete(t.usedNonces, nonce)
		}
	}
}

// Stop stops the cleanup loop.
func (t *NonceTracker) Stop() {
	close(t.stopChan)
}