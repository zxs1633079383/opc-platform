package goal

import "testing"

func TestIsInteractivePrompt(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Here is the login page design with full mockups", false},
		{"Want to try it? I can show mockups in a browser", true},
		{"Would you like me to proceed?", true},
		{"Shall I create the file?", true},
		{"Do you want me to continue?", true},
		{"需要你的确认才能继续", true},
		{"是否继续执行？", true},
		{"你想试试吗？", true},
		{"Requires opening a local URL", true},
		{"I've completed the login page implementation with all components", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 30)], func(t *testing.T) {
			got := isInteractivePrompt(tt.input)
			if got != tt.expected {
				t.Errorf("isInteractivePrompt(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAssessResult_EmptyResult(t *testing.T) {
	gd := &GoalDriver{logger: testLogger()}
	assessment, err := gd.AssessResult(nil, "Login Feature", "Design login page", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assessment.Satisfied {
		t.Error("empty result should not be satisfied")
	}
	if assessment.FollowUp == "" {
		t.Error("should have followUp instruction for empty result")
	}
}

func TestAssessResult_InteractivePrompt(t *testing.T) {
	gd := &GoalDriver{logger: testLogger()}
	result := "I can put together mockups. Want to try it? Requires opening a local URL"
	assessment, err := gd.AssessResult(nil, "Login Feature", "Design login page", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assessment.Satisfied {
		t.Error("interactive prompt should not be satisfied")
	}
	if assessment.FollowUp == "" {
		t.Error("should have followUp for interactive prompt")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
