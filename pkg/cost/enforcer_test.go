package cost

import (
	"testing"

	"go.uber.org/zap"
)

func testLogger() *zap.SugaredLogger {
	l, _ := zap.NewDevelopment()
	return l.Sugar()
}

func TestQuotaEnforcer_NoConfig(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())
	result := e.Check("agent1", QuotaConfig{})
	if !result.Allowed {
		t.Error("expected allowed when no config")
	}
}

func TestQuotaEnforcer_DailyTokenExceeded(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())

	// Record 1000 tokens.
	e.RecordUsage("agent1", 500, 500, 0.01)

	config := QuotaConfig{
		TokenPerDay: 1000,
		OnExceed:    ExceedReject,
	}

	result := e.Check("agent1", config)
	if result.Allowed {
		t.Error("expected rejected when daily token budget exceeded")
	}
	if result.Action != ExceedReject {
		t.Errorf("expected reject action, got %s", result.Action)
	}
}

func TestQuotaEnforcer_DailyTokenAlert(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())

	// Record 850 tokens out of 1000 (85% > 80% threshold).
	e.RecordUsage("agent1", 425, 425, 0.01)

	config := QuotaConfig{
		TokenPerDay: 1000,
	}

	result := e.Check("agent1", config)
	if !result.Allowed {
		t.Error("expected allowed (alert only)")
	}
	if result.AlertMsg == "" {
		t.Error("expected alert message at 85%")
	}
}

func TestQuotaEnforcer_HourlyTokenExceeded(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())
	e.RecordUsage("agent1", 300, 300, 0)

	config := QuotaConfig{
		TokenPerHour: 500,
		OnExceed:     ExceedPause,
	}

	result := e.Check("agent1", config)
	if result.Allowed {
		t.Error("expected not allowed")
	}
	if result.Action != ExceedPause {
		t.Errorf("expected pause, got %s", result.Action)
	}
}

func TestQuotaEnforcer_CostPerDayExceeded(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())
	e.RecordUsage("agent1", 0, 0, 5.0)

	config := QuotaConfig{
		CostPerDay: 5.0,
		OnExceed:   ExceedReject,
	}

	result := e.Check("agent1", config)
	if result.Allowed {
		t.Error("expected not allowed when daily cost exceeded")
	}
}

func TestQuotaEnforcer_CostPerMonthExceeded(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())
	e.RecordUsage("agent1", 0, 0, 100.0)

	config := QuotaConfig{
		CostPerMonth: 100.0,
		OnExceed:     ExceedReject,
	}

	result := e.Check("agent1", config)
	if result.Allowed {
		t.Error("expected not allowed when monthly cost exceeded")
	}
}

func TestQuotaEnforcer_AlertMode(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())
	e.RecordUsage("agent1", 500, 500, 0)

	config := QuotaConfig{
		TokenPerDay: 500,
		OnExceed:    ExceedAlert,
	}

	// Even though exceeded, alert mode should NOT block.
	// Check returns Allowed=false but Action=ExceedAlert, so the caller skips rejection.
	result := e.Check("agent1", config)
	if result.Allowed {
		t.Error("expected Allowed=false when exceeded")
	}
	if result.Action != ExceedAlert {
		t.Errorf("expected alert action, got %s", result.Action)
	}
}

func TestQuotaEnforcer_MultiAgent(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())

	e.RecordUsage("agent1", 500, 500, 1.0)
	e.RecordUsage("agent2", 100, 100, 0.1)

	config := QuotaConfig{
		TokenPerDay: 500,
		OnExceed:    ExceedReject,
	}

	// Agent1 exceeded.
	r1 := e.Check("agent1", config)
	if r1.Allowed {
		t.Error("agent1 should be blocked")
	}

	// Agent2 should be fine.
	r2 := e.Check("agent2", config)
	if !r2.Allowed {
		t.Error("agent2 should be allowed")
	}
}

func TestQuotaEnforcer_GetUsage(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())
	e.RecordUsage("agent1", 100, 200, 0.5)

	ht, dt, dc, mc := e.GetUsage("agent1")
	if ht != 300 || dt != 300 {
		t.Errorf("expected hourly=300 daily=300, got hourly=%d daily=%d", ht, dt)
	}
	if dc != 0.5 || mc != 0.5 {
		t.Errorf("expected dailyCost=0.5 monthlyCost=0.5, got daily=%.4f monthly=%.4f", dc, mc)
	}

	// Unknown agent returns zeros.
	ht2, dt2, dc2, mc2 := e.GetUsage("unknown")
	if ht2 != 0 || dt2 != 0 || dc2 != 0 || mc2 != 0 {
		t.Error("expected zeros for unknown agent")
	}
}

func TestParseCostString(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"$1.00", 1.0},
		{"$0.50", 0.5},
		{"1.5", 1.5},
		{"$100", 100.0},
		{"", 0},
		{" $2.50 ", 2.5},
	}
	for _, tt := range tests {
		got := ParseCostString(tt.input)
		if got != tt.want {
			t.Errorf("ParseCostString(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestQuotaEnforcer_DefaultAction(t *testing.T) {
	e := NewQuotaEnforcer(testLogger())
	e.RecordUsage("agent1", 500, 500, 0)

	// No OnExceed set — should default to reject.
	config := QuotaConfig{
		TokenPerDay: 500,
	}
	result := e.Check("agent1", config)
	if result.Action != ExceedReject {
		t.Errorf("expected default reject action, got %s", result.Action)
	}
}
