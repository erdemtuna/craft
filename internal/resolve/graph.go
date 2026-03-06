package resolve

import "fmt"

// Graph is a directed dependency graph used for cycle detection
// and topological ordering.
type Graph struct {
	// nodes maps node ID to its adjacency list (outgoing edges).
	nodes map[string][]string
}

// NewGraph creates an empty dependency graph.
func NewGraph() *Graph {
	return &Graph{nodes: make(map[string][]string)}
}

// AddNode ensures a node exists in the graph.
func (g *Graph) AddNode(id string) {
	if _, ok := g.nodes[id]; !ok {
		g.nodes[id] = nil
	}
}

// AddEdge adds a directed edge from → to.
func (g *Graph) AddEdge(from, to string) {
	g.AddNode(from)
	g.AddNode(to)
	g.nodes[from] = append(g.nodes[from], to)
}

// DetectCycles uses DFS to find cycles. Returns the cycle path if found,
// or nil if the graph is acyclic.
func (g *Graph) DetectCycles() []string {
	const (
		white = iota // not visited
		gray         // in current DFS path
		black        // fully explored
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	for id := range g.nodes {
		color[id] = white
	}

	var dfs func(node string) []string
	dfs = func(node string) []string {
		color[node] = gray

		for _, neighbor := range g.nodes[node] {
			if color[neighbor] == gray {
				// Found cycle — reconstruct path
				cycle := []string{neighbor, node}
				curr := node
				for curr != neighbor {
					curr = parent[curr]
					cycle = append(cycle, curr)
				}
				// Reverse to get forward order
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return cycle
			}
			if color[neighbor] == white {
				parent[neighbor] = node
				if cycle := dfs(neighbor); cycle != nil {
					return cycle
				}
			}
		}

		color[node] = black
		return nil
	}

	for id := range g.nodes {
		if color[id] == white {
			if cycle := dfs(id); cycle != nil {
				return cycle
			}
		}
	}

	return nil
}

// FormatCycle produces a human-readable cycle path string.
func FormatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	result := ""
	for i, node := range cycle {
		if i > 0 {
			result += " → "
		}
		result += node
	}
	return fmt.Sprintf("dependency cycle detected: %s", result)
}
