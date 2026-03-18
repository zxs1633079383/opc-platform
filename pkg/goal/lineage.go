package goal

import "encoding/json"

// LineageRef tracks the provenance of an issue through the federation chain.
type LineageRef struct {
	GoalID      string `json:"goalId"`
	ProjectName string `json:"projectName"`
	IssueID     string `json:"issueId"`
	OPCNode     string `json:"opcNode"`
	Label       string `json:"label"`
}

// AppendLineage returns a new slice with ref appended, without mutating upstream.
func AppendLineage(upstream []LineageRef, ref LineageRef) []LineageRef {
	result := make([]LineageRef, len(upstream)+1)
	copy(result, upstream)
	result[len(upstream)] = ref
	return result
}

// LineageToJSON serializes a lineage chain to JSON. Returns "[]" for empty/nil input.
func LineageToJSON(refs []LineageRef) (string, error) {
	if len(refs) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(refs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// LineageFromJSON deserializes a JSON string into a lineage chain. Returns nil for empty input.
func LineageFromJSON(data string) ([]LineageRef, error) {
	if data == "" || data == "[]" {
		return nil, nil
	}
	var refs []LineageRef
	if err := json.Unmarshal([]byte(data), &refs); err != nil {
		return nil, err
	}
	return refs, nil
}
