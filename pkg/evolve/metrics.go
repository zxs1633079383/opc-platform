package evolve

import "time"

// SystemMetrics holds system-wide performance metrics.
type SystemMetrics struct {
	SuccessRate   float64  `json:"successRate"`
	AvgLatency    float64  `json:"avgLatency"`     // seconds
	CostPerGoal   float64  `json:"costPerGoal"`
	RetryRate     float64  `json:"retryRate"`
	CoverageGap   float64  `json:"coverageGap"`
	ErrorPatterns []string `json:"errorPatterns"`
	Timestamp     time.Time `json:"timestamp"`
}

// MetricsStore defines the interface for querying task/goal data.
type MetricsStore interface {
	TaskSuccessRate() (float64, error)
	TaskAvgLatency() (float64, error)
	GoalAvgCost() (float64, error)
	TaskRetryRate() (float64, error)
}

// MetricsCollector gathers system metrics from the store.
type MetricsCollector struct {
	store MetricsStore
}

// NewMetricsCollector creates a new MetricsCollector backed by the given store.
func NewMetricsCollector(store MetricsStore) *MetricsCollector {
	return &MetricsCollector{store: store}
}

// Collect gathers current system metrics from the underlying store.
func (mc *MetricsCollector) Collect() (*SystemMetrics, error) {
	sr, _ := mc.store.TaskSuccessRate()
	lat, _ := mc.store.TaskAvgLatency()
	cpg, _ := mc.store.GoalAvgCost()
	rr, _ := mc.store.TaskRetryRate()

	return &SystemMetrics{
		SuccessRate: sr,
		AvgLatency:  lat,
		CostPerGoal: cpg,
		RetryRate:   rr,
		Timestamp:   time.Now(),
	}, nil
}
