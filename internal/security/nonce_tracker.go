package security

import (
	"sync"
	"time"
)

// NonceTracker tracks used nonces to prevent replay attacks
type NonceTracker struct {
	usedNonces map[string]time.Time
	mu         sync.RWMutex
	cleanupInterval time.Duration
	stopChan   chan struct{}
}

// NewNonceTracker creates a new nonce tracker
func NewNonceTracker(cleanupInterval time.Duration) *NonceTracker {
	tracker := &NonceTracker{
		usedNonces: make(map[string]time.Time),
		cleanupInterval: cleanupInterval,
		stopChan: make(chan struct{}),
	}
	
	go tracker.cleanupLoop()
	return tracker
}

// Add adds a nonce to the tracker
func (t *NonceTracker) Add(nonce string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if _, exists := t.usedNonces[nonce]; exists {
		return false // Nonce already used
	}
	
	t.usedNonces[nonce] = time.Now()
	return true
}

// CheckAndAdd checks if a nonce has been used, and adds it if not
func (t *NonceTracker) CheckAndAdd(nonce string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if _, exists := t.usedNonces[nonce]; exists {
		return false // Nonce already used
	}
	
	t.usedNonces[nonce] = time.Now()
	return true
}

// cleanupLoop periodically removes old nonces
func (t *NonceTracker) cleanupLoop() {
	ticker := time.NewTicker(t.cleanupInterval)
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

// cleanup removes nonces older than 24 hours
func (t *NonceTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	cutoff := time.Now().Add(-24 * time.Hour)
	for nonce, timestamp := range t.usedNonces {
		if timestamp.Before(cutoff) {
			delete(t.usedNonces, nonce)
		}
	}
}

// Stop stops the cleanup loop
func (t *NonceTracker) Stop() {
	close(t.stopChan)
}