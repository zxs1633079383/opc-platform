package federation

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RetryEntry represents a single retry-able HTTP request.
type RetryEntry struct {
	ID          string
	URL         string
	Payload     []byte
	Attempts    int
	MaxAttempts int
	NextRetry   time.Time
	CreatedAt   time.Time
}

// RetryQueue manages a queue of failed HTTP requests for retry with
// exponential backoff.
type RetryQueue struct {
	mu      sync.Mutex
	entries []*RetryEntry
	logger  *zap.SugaredLogger
}

// NewRetryQueue creates a new RetryQueue.
func NewRetryQueue(logger *zap.SugaredLogger) *RetryQueue {
	return &RetryQueue{logger: logger}
}

// Enqueue adds a failed request to the retry queue.
func (q *RetryQueue) Enqueue(url string, payload []byte) {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry := &RetryEntry{
		ID:          fmt.Sprintf("retry-%d", time.Now().UnixNano()),
		URL:         url,
		Payload:     payload,
		MaxAttempts: 10,
		NextRetry:   time.Now().Add(time.Second),
		CreatedAt:   time.Now(),
	}
	q.entries = append(q.entries, entry)
	q.logger.Infow("Enqueue", "id", entry.ID, "url", url, "payloadSize", len(payload), "queueLen", len(q.entries))
}

// ProcessLoop runs a background loop that retries queued requests every 5 seconds.
func (q *RetryQueue) ProcessLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.processOnce()
		}
	}
}

func (q *RetryQueue) processOnce() {
	q.mu.Lock()
	now := time.Now()
	var remaining []*RetryEntry
	var ready []*RetryEntry
	for _, e := range q.entries {
		if e.NextRetry.Before(now) {
			ready = append(ready, e)
		} else {
			remaining = append(remaining, e)
		}
	}
	q.entries = remaining
	q.mu.Unlock()

	for _, e := range ready {
		if err := q.send(e); err != nil {
			e.Attempts++
			if e.Attempts >= e.MaxAttempts {
				q.logger.Warnw("retry exhausted, dropping", "id", e.ID, "url", e.URL, "totalAttempts", e.Attempts)
				continue
			}
			backoff := time.Duration(1<<uint(e.Attempts)) * time.Second
			if backoff > 5*time.Minute {
				backoff = 5 * time.Minute
			}
			e.NextRetry = time.Now().Add(backoff)
			q.mu.Lock()
			q.entries = append(q.entries, e)
			q.mu.Unlock()
			q.logger.Warnw("retry failed, rescheduled",
				"id", e.ID, "url", e.URL, "attempt", e.Attempts,
				"remaining", e.MaxAttempts-e.Attempts, "nextRetry", e.NextRetry, "error", err)
		} else {
			q.logger.Infow("retry succeeded", "id", e.ID, "url", e.URL, "attempt", e.Attempts)
		}
	}
}

func (q *RetryQueue) send(e *RetryEntry) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(e.URL, "application/json", bytes.NewReader(e.Payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// Len returns the number of entries in the queue.
func (q *RetryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
}
