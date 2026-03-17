package federation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- TestRetryQueue_Enqueue ---

func TestRetryQueue_Enqueue(t *testing.T) {
	q := NewRetryQueue(newTestLogger())

	if q.Len() != 0 {
		t.Fatalf("expected empty queue, got %d", q.Len())
	}

	q.Enqueue("http://example.com/api", []byte(`{"data": "test"}`))
	if q.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", q.Len())
	}

	q.Enqueue("http://example.com/api/2", []byte(`{"data": "test2"}`))
	if q.Len() != 2 {
		t.Errorf("expected 2 entries, got %d", q.Len())
	}
}

// --- TestRetryQueue_ProcessLoop_Success ---

func TestRetryQueue_ProcessLoop_Success(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	q := NewRetryQueue(newTestLogger())

	// Set NextRetry to the past so it triggers immediately.
	q.mu.Lock()
	q.entries = append(q.entries, &RetryEntry{
		ID:          "test-1",
		URL:         server.URL,
		Payload:     []byte(`{"ok": true}`),
		MaxAttempts: 10,
		NextRetry:   time.Now().Add(-time.Second),
		CreatedAt:   time.Now(),
	})
	q.mu.Unlock()

	// Process once.
	q.processOnce()

	select {
	case body := <-received:
		if string(body) != `{"ok": true}` {
			t.Errorf("unexpected body: %s", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request")
	}

	// After success, queue should be empty.
	if q.Len() != 0 {
		t.Errorf("expected empty queue after success, got %d", q.Len())
	}
}

// --- TestRetryQueue_ProcessLoop_RetryWithBackoff ---

func TestRetryQueue_ProcessLoop_RetryWithBackoff(t *testing.T) {
	// Server that always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	q := NewRetryQueue(newTestLogger())

	q.mu.Lock()
	q.entries = append(q.entries, &RetryEntry{
		ID:          "retry-test",
		URL:         server.URL,
		Payload:     []byte(`{"retry": true}`),
		Attempts:    0,
		MaxAttempts: 10,
		NextRetry:   time.Now().Add(-time.Second),
		CreatedAt:   time.Now(),
	})
	q.mu.Unlock()

	// Process once — should fail and re-enqueue with increased attempts.
	q.processOnce()

	if q.Len() != 1 {
		t.Fatalf("expected 1 entry after failed retry, got %d", q.Len())
	}

	q.mu.Lock()
	entry := q.entries[0]
	q.mu.Unlock()

	if entry.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", entry.Attempts)
	}
	if entry.NextRetry.Before(time.Now()) {
		t.Error("next retry should be in the future after backoff")
	}
}

// --- TestRetryQueue_MaxAttempts ---

func TestRetryQueue_MaxAttempts(t *testing.T) {
	// Server that always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	q := NewRetryQueue(newTestLogger())

	q.mu.Lock()
	q.entries = append(q.entries, &RetryEntry{
		ID:          "exhaust-test",
		URL:         server.URL,
		Payload:     []byte(`{"exhaust": true}`),
		Attempts:    9, // MaxAttempts is 10, so this is the last attempt.
		MaxAttempts: 10,
		NextRetry:   time.Now().Add(-time.Second),
		CreatedAt:   time.Now(),
	})
	q.mu.Unlock()

	q.processOnce()

	// Should be dropped (not re-enqueued) because attempts >= maxAttempts.
	if q.Len() != 0 {
		t.Errorf("expected empty queue after max attempts, got %d", q.Len())
	}
}

// --- TestRetryQueue_ProcessLoop_ContextCancel ---

func TestRetryQueue_ProcessLoop_ContextCancel(t *testing.T) {
	q := NewRetryQueue(newTestLogger())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		q.ProcessLoop(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// ProcessLoop exited.
	case <-time.After(2 * time.Second):
		t.Fatal("ProcessLoop did not exit after context cancel")
	}
}

// --- TestRetryQueue_Enqueue_FieldValues ---

func TestRetryQueue_Enqueue_FieldValues(t *testing.T) {
	q := NewRetryQueue(newTestLogger())
	q.Enqueue("http://example.com/test", []byte("payload"))

	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(q.entries))
	}

	entry := q.entries[0]
	if entry.URL != "http://example.com/test" {
		t.Errorf("unexpected URL: %s", entry.URL)
	}
	if string(entry.Payload) != "payload" {
		t.Errorf("unexpected payload: %s", entry.Payload)
	}
	if entry.MaxAttempts != 10 {
		t.Errorf("expected MaxAttempts 10, got %d", entry.MaxAttempts)
	}
	if entry.Attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", entry.Attempts)
	}
	if entry.ID == "" {
		t.Error("expected non-empty ID")
	}
}
