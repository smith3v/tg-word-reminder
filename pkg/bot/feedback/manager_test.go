package feedback

import (
	"testing"
	"time"
)

func TestManagerStartOverwrite(t *testing.T) {
	manager := NewManager(nil)
	start := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)

	manager.Start(1, 10, start, 5*time.Minute)
	manager.Start(1, 10, start.Add(time.Minute), 5*time.Minute)

	manager.mu.Lock()
	entry, ok := manager.pending[1]
	manager.mu.Unlock()
	if !ok {
		t.Fatalf("expected pending feedback to exist")
	}
	expected := start.Add(time.Minute).Add(5 * time.Minute)
	if !entry.ExpiresAt.Equal(expected) {
		t.Fatalf("expected expires at %v, got %v", expected, entry.ExpiresAt)
	}
}

func TestManagerConsumeBeforeExpiration(t *testing.T) {
	manager := NewManager(nil)
	start := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)

	manager.Start(1, 10, start, 5*time.Minute)
	if ok := manager.Consume(1, 10, start.Add(time.Minute)); !ok {
		t.Fatalf("expected consume to succeed before expiration")
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.pending) != 0 {
		t.Fatalf("expected pending feedback to be cleared")
	}
}

func TestManagerConsumeAfterExpiration(t *testing.T) {
	manager := NewManager(nil)
	start := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)

	manager.Start(1, 10, start, 5*time.Minute)
	if ok := manager.Consume(1, 10, start.Add(5*time.Minute)); ok {
		t.Fatalf("expected consume to fail after expiration")
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.pending) != 0 {
		t.Fatalf("expected pending feedback to be cleared after expiration")
	}
}

func TestManagerSweepExpired(t *testing.T) {
	manager := NewManager(nil)
	start := time.Date(2026, 1, 23, 12, 0, 0, 0, time.UTC)

	manager.Start(1, 10, start, 5*time.Minute)
	manager.Start(2, 20, start, 10*time.Minute)

	manager.SweepExpired(start.Add(6 * time.Minute))

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, ok := manager.pending[1]; ok {
		t.Fatalf("expected expired entry to be removed")
	}
	if _, ok := manager.pending[2]; !ok {
		t.Fatalf("expected unexpired entry to remain")
	}
}
