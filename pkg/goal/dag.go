package goal

import "fmt"

// ValidateProjectDAG checks that all project dependencies exist and there are no cycles.
func ValidateProjectDAG(projects []*Project) error {
	nameSet := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		if p.Name == "" {
			return fmt.Errorf("project name is required")
		}
		if _, dup := nameSet[p.Name]; dup {
			return fmt.Errorf("duplicate project name: %q", p.Name)
		}
		nameSet[p.Name] = struct{}{}
	}

	for _, p := range projects {
		for _, dep := range p.Dependencies {
			if _, ok := nameSet[dep]; !ok {
				return fmt.Errorf("project %q depends on unknown project %q", p.Name, dep)
			}
			if dep == p.Name {
				return fmt.Errorf("project %q depends on itself", p.Name)
			}
		}
	}

	// Kahn's algorithm for cycle detection.
	inDegree := make(map[string]int, len(projects))
	adjacency := make(map[string][]string, len(projects))
	for _, p := range projects {
		if _, ok := inDegree[p.Name]; !ok {
			inDegree[p.Name] = 0
		}
		for _, dep := range p.Dependencies {
			adjacency[dep] = append(adjacency[dep], p.Name)
			inDegree[p.Name]++
		}
	}

	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adjacency[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != len(projects) {
		return fmt.Errorf("project dependencies contain a cycle")
	}
	return nil
}

// BuildProjectLayers performs topological sort and groups projects into layers.
// Projects within the same layer have no interdependencies and can be dispatched in parallel.
func BuildProjectLayers(projects []*Project) ([][]*Project, error) {
	projectMap := make(map[string]*Project, len(projects))
	inDegree := make(map[string]int, len(projects))
	adjacency := make(map[string][]string, len(projects))

	for _, p := range projects {
		projectMap[p.Name] = p
		if _, ok := inDegree[p.Name]; !ok {
			inDegree[p.Name] = 0
		}
		for _, dep := range p.Dependencies {
			adjacency[dep] = append(adjacency[dep], p.Name)
			inDegree[p.Name]++
		}
	}

	var layers [][]*Project

	for {
		var currentLayer []*Project
		for name, deg := range inDegree {
			if deg == 0 {
				currentLayer = append(currentLayer, projectMap[name])
			}
		}

		if len(currentLayer) == 0 {
			break
		}

		for _, p := range currentLayer {
			delete(inDegree, p.Name)
			for _, next := range adjacency[p.Name] {
				inDegree[next]--
			}
		}

		layers = append(layers, currentLayer)
	}

	if len(inDegree) > 0 {
		return nil, fmt.Errorf("cycle detected in project dependencies")
	}

	return layers, nil
}
