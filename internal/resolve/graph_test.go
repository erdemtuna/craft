package resolve

import (
	"strings"
	"testing"
)

func TestGraphAcyclic(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C")

	cycle := g.DetectCycles()
	if cycle != nil {
		t.Errorf("Expected no cycle, got %v", cycle)
	}
}

func TestGraphSingleCycle(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "A")

	cycle := g.DetectCycles()
	if cycle == nil {
		t.Fatal("Expected cycle")
	}
	if len(cycle) < 2 {
		t.Errorf("Cycle should have at least 2 nodes, got %v", cycle)
	}
}

func TestGraphThreeNodeCycle(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")

	cycle := g.DetectCycles()
	if cycle == nil {
		t.Fatal("Expected cycle")
	}
	// Should contain all three nodes
	has := map[string]bool{}
	for _, n := range cycle {
		has[n] = true
	}
	for _, node := range []string{"A", "B", "C"} {
		if !has[node] {
			t.Errorf("Cycle should include %q, got %v", node, cycle)
		}
	}
}

func TestGraphDiamond(t *testing.T) {
	// Diamond: A → B, A → C, B → D, C → D
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "D")
	g.AddEdge("C", "D")

	cycle := g.DetectCycles()
	if cycle != nil {
		t.Errorf("Diamond should not be a cycle, got %v", cycle)
	}
}

func TestGraphSelfLoop(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "A")

	cycle := g.DetectCycles()
	if cycle == nil {
		t.Fatal("Expected self-loop cycle")
	}
}

func TestGraphIsolatedNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A")
	g.AddNode("B")

	cycle := g.DetectCycles()
	if cycle != nil {
		t.Errorf("Isolated nodes should not form cycle, got %v", cycle)
	}
}

func TestFormatCycle(t *testing.T) {
	result := FormatCycle([]string{"A", "B", "C", "A"})
	if !strings.Contains(result, "A → B → C → A") {
		t.Errorf("FormatCycle should produce arrow-separated path, got %q", result)
	}
	if !strings.Contains(result, "dependency cycle") {
		t.Errorf("FormatCycle should mention cycle, got %q", result)
	}
}

func TestFormatCycleEmpty(t *testing.T) {
	result := FormatCycle(nil)
	if result != "" {
		t.Errorf("FormatCycle(nil) should be empty, got %q", result)
	}
}
