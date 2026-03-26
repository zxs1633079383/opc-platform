package evolve

import (
	"testing"
	"time"
)

// mockStore implements MetricsStore for testing.
type mockStore struct {
	successRate float64
	avgLatency  float64
	avgCost     float64
	retryRate   float64
}

func (m *mockStore) TaskSuccessRate() (float64, error) { return m.successRate, nil }
func (m *mockStore) TaskAvgLatency() (float64, error)  { return m.avgLatency, nil }
func (m *mockStore) GoalAvgCost() (float64, error)     { return m.avgCost, nil }
func (m *mockStore) TaskRetryRate() (float64, error)    { return m.retryRate, nil }

func TestMetricsCollector_Collect(t *testing.T) {
	store := &mockStore{
		successRate: 0.95,
		avgLatency:  2.5,
		avgCost:     1.20,
		retryRate:   0.05,
	}
	mc := NewMetricsCollector(store)

	metrics, err := mc.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.SuccessRate != 0.95 {
		t.Errorf("expected success rate 0.95, got %f", metrics.SuccessRate)
	}
	if metrics.AvgLatency != 2.5 {
		t.Errorf("expected avg latency 2.5, got %f", metrics.AvgLatency)
	}
	if metrics.CostPerGoal != 1.20 {
		t.Errorf("expected cost per goal 1.20, got %f", metrics.CostPerGoal)
	}
	if metrics.RetryRate != 0.05 {
		t.Errorf("expected retry rate 0.05, got %f", metrics.RetryRate)
	}
	if metrics.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if time.Since(metrics.Timestamp) > time.Second {
		t.Error("timestamp should be recent")
	}
}

func TestMetricsCollector_CollectZeroValues(t *testing.T) {
	store := &mockStore{}
	mc := NewMetricsCollector(store)

	metrics, err := mc.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.SuccessRate != 0 {
		t.Errorf("expected 0, got %f", metrics.SuccessRate)
	}
	if metrics.AvgLatency != 0 {
		t.Errorf("expected 0, got %f", metrics.AvgLatency)
	}
}
