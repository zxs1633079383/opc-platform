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

// v0.7: Smart retry category tests.

func TestAssessResult_EmptyResult_Category(t *testing.T) {
	gd := &GoalDriver{logger: testLogger()}
	assessment, _ := gd.AssessResult(nil, "Login Feature", "Design login page", "")
	if assessment.Category != CategoryEmptyResult {
		t.Errorf("expected CategoryEmptyResult, got %s", assessment.Category)
	}
}

func TestAssessResult_VerificationTask_EmptyAccepted(t *testing.T) {
	gd := &GoalDriver{logger: testLogger()}
	// A verification task with empty result should be accepted.
	assessment, _ := gd.AssessResult(nil, "CI Pipeline", "检查代码是否有安全漏洞", "")
	if !assessment.Satisfied {
		t.Error("verification task with empty result should be satisfied")
	}
	if assessment.Category != CategorySatisfied {
		t.Errorf("expected CategorySatisfied, got %s", assessment.Category)
	}
}

func TestAssessResult_ExecutionError_Category(t *testing.T) {
	gd := &GoalDriver{logger: testLogger()}
	result := "Error: failed to compile main.go\npanic: runtime error: index out of range"
	assessment, _ := gd.AssessResult(nil, "Build", "Build the project", result)
	if assessment.Satisfied {
		t.Error("execution error should not be satisfied")
	}
	if assessment.Category != CategoryExecutionError {
		t.Errorf("expected CategoryExecutionError, got %s", assessment.Category)
	}
}

func TestAssessResult_InteractivePrompt_Category(t *testing.T) {
	gd := &GoalDriver{logger: testLogger()}
	result := "Would you like me to proceed with the implementation?"
	assessment, _ := gd.AssessResult(nil, "Feature", "Implement feature", result)
	if assessment.Category != CategoryQualityIssue {
		t.Errorf("expected CategoryQualityIssue, got %s", assessment.Category)
	}
}

func TestResultCategory_MaxRetries(t *testing.T) {
	tests := []struct {
		cat  ResultCategory
		want int
	}{
		{CategoryEmptyResult, 1},
		{CategoryExecutionError, 2},
		{CategoryQualityIssue, 3},
		{CategorySatisfied, 0},
	}
	for _, tt := range tests {
		got := tt.cat.MaxRetries()
		if got != tt.want {
			t.Errorf("%s.MaxRetries() = %d, want %d", tt.cat, got, tt.want)
		}
	}
}

func TestIsVerificationTask(t *testing.T) {
	tests := []struct {
		desc string
		want bool
	}{
		{"检查代码是否有安全漏洞", true},
		{"verify the build passes", true},
		{"run lint on the codebase", true},
		{"Design the login page", false},
		{"Implement user authentication", false},
		{"validate API responses", true},
	}
	for _, tt := range tests {
		got := isVerificationTask(tt.desc)
		if got != tt.want {
			t.Errorf("isVerificationTask(%q) = %v, want %v", tt.desc, got, tt.want)
		}
	}
}

func TestIsExecutionError(t *testing.T) {
	tests := []struct {
		result string
		want   bool
	}{
		{"Error: failed to compile", true},
		{"panic: runtime error", true},
		{"Permission denied", true},
		{"Here is the completed implementation", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isExecutionError(tt.result)
		if got != tt.want {
			t.Errorf("isExecutionError(%q) = %v, want %v", tt.result, got, tt.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
