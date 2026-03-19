package goal

import (
	"testing"
)

func TestValidateProjectDAG_NoCycle(t *testing.T) {
	projects := []*Project{
		{Name: "ui-design"},
		{Name: "api-design"},
		{Name: "frontend", Dependencies: []string{"ui-design", "api-design"}},
		{Name: "testing", Dependencies: []string{"frontend"}},
	}
	if err := ValidateProjectDAG(projects); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateProjectDAG_Cycle(t *testing.T) {
	projects := []*Project{
		{Name: "a", Dependencies: []string{"b"}},
		{Name: "b", Dependencies: []string{"a"}},
	}
	if err := ValidateProjectDAG(projects); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateProjectDAG_SelfDep(t *testing.T) {
	projects := []*Project{
		{Name: "a", Dependencies: []string{"a"}},
	}
	if err := ValidateProjectDAG(projects); err == nil {
		t.Fatal("expected self-dependency error")
	}
}

func TestValidateProjectDAG_MissingDep(t *testing.T) {
	projects := []*Project{
		{Name: "a", Dependencies: []string{"nonexistent"}},
	}
	if err := ValidateProjectDAG(projects); err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestValidateProjectDAG_DuplicateName(t *testing.T) {
	projects := []*Project{
		{Name: "a"},
		{Name: "a"},
	}
	if err := ValidateProjectDAG(projects); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestBuildProjectLayers_LinearChain(t *testing.T) {
	projects := []*Project{
		{Name: "a"},
		{Name: "b", Dependencies: []string{"a"}},
		{Name: "c", Dependencies: []string{"b"}},
	}
	layers, err := BuildProjectLayers(projects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(layers))
	}
	if layers[0][0].Name != "a" {
		t.Errorf("layer 0: expected 'a', got %q", layers[0][0].Name)
	}
	if layers[1][0].Name != "b" {
		t.Errorf("layer 1: expected 'b', got %q", layers[1][0].Name)
	}
	if layers[2][0].Name != "c" {
		t.Errorf("layer 2: expected 'c', got %q", layers[2][0].Name)
	}
}

func TestBuildProjectLayers_DiamondDependency(t *testing.T) {
	// ui-design ──┐
	//             ├──► frontend ──► testing
	// api-design ─┘
	projects := []*Project{
		{Name: "ui-design"},
		{Name: "api-design"},
		{Name: "frontend", Dependencies: []string{"ui-design", "api-design"}},
		{Name: "testing", Dependencies: []string{"frontend"}},
	}
	layers, err := BuildProjectLayers(projects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(layers))
	}
	// Layer 0: ui-design and api-design (parallel)
	if len(layers[0]) != 2 {
		t.Errorf("layer 0: expected 2 projects, got %d", len(layers[0]))
	}
	// Layer 1: frontend
	if len(layers[1]) != 1 || layers[1][0].Name != "frontend" {
		t.Errorf("layer 1: expected [frontend], got %v", layerNames(layers[1]))
	}
	// Layer 2: testing
	if len(layers[2]) != 1 || layers[2][0].Name != "testing" {
		t.Errorf("layer 2: expected [testing], got %v", layerNames(layers[2]))
	}
}

func TestBuildProjectLayers_AllParallel(t *testing.T) {
	projects := []*Project{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	layers, err := BuildProjectLayers(projects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer (all parallel), got %d", len(layers))
	}
	if len(layers[0]) != 3 {
		t.Errorf("expected 3 projects in layer 0, got %d", len(layers[0]))
	}
}

func layerNames(layer []*Project) []string {
	names := make([]string, len(layer))
	for i, p := range layer {
		names[i] = p.Name
	}
	return names
}
