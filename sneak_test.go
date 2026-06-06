package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		target string
		query  string
		want   bool
	}{
		{"", "", true},
		{"abc", "", true},
		{"abc", "a", true},
		{"abc", "ac", true},
		{"abc", "abcd", false},
		{"config", "cfg", true},
		{"Downloads", "dl", true},
		{"Downloads", "down", true},
		{"Downloads", "lx", false}, // no 'x' in Downloads
	}

	for _, tt := range tests {
		got := fuzzyMatch(tt.target, tt.query)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v; want %v", tt.target, tt.query, got, tt.want)
		}
	}
}

func TestFuzzyMatchWithIndices(t *testing.T) {
	tests := []struct {
		target      string
		query       string
		wantMatched bool
		wantIndices []int
	}{
		{"", "", true, nil},
		{"abc", "ac", true, []int{0, 2}},
		{"config", "cfg", true, []int{0, 3, 5}},
		{"Downloads", "dl", true, []int{0, 4}},
		{"Downloads", "lx", false, nil},
	}

	for _, tt := range tests {
		matched, indices := fuzzyMatchWithIndices(tt.target, tt.query)
		if matched != tt.wantMatched {
			t.Errorf("fuzzyMatchWithIndices(%q, %q) matched = %v; want %v", tt.target, tt.query, matched, tt.wantMatched)
		}
		if len(indices) != len(tt.wantIndices) {
			t.Errorf("fuzzyMatchWithIndices(%q, %q) indices = %v; want %v", tt.target, tt.query, indices, tt.wantIndices)
			continue
		}
		for i := range indices {
			if indices[i] != tt.wantIndices[i] {
				t.Errorf("fuzzyMatchWithIndices(%q, %q) indices[%d] = %v; want %v", tt.target, tt.query, i, indices[i], tt.wantIndices[i])
			}
		}
	}
}

func TestTreeFlatteningAndNavigation(t *testing.T) {
	// Setup a temporary directory hierarchy
	tmpDir, err := os.MkdirTemp("", "sneak-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directories:
	// tmpDir/
	//   dirA/
	//     dirA1/
	//     dirA2/
	//   dirB/
	dirA := filepath.Join(tmpDir, "dirA")
	dirA1 := filepath.Join(dirA, "dirA1")
	dirA2 := filepath.Join(dirA, "dirA2")
	dirB := filepath.Join(tmpDir, "dirB")

	for _, d := range []string{dirA, dirA1, dirA2, dirB} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	// Initialize root node
	root := NewNode(tmpDir, nil)
	if err := root.LoadChildren(); err != nil {
		t.Fatalf("LoadChildren failed: %v", err)
	}

	if len(root.Children) != 2 {
		t.Errorf("expected 2 children, got %v", len(root.Children))
	}

	root.Expanded = true
	// Expand dirA
	var nodeA *Node
	for _, child := range root.Children {
		if child.Name == "dirA" {
			nodeA = child
			break
		}
	}

	if nodeA == nil {
		t.Fatal("dirA not found in children")
	}

	nodeA.Expanded = true
	if err := nodeA.LoadChildren(); err != nil {
		t.Fatalf("failed to load children of dirA: %v", err)
	}

	// Flatten visible nodes
	visible := root.GetVisibleNodes(nil, "")
	// Expected visible: root, dirA, dirA1, dirA2, dirB
	// (Note: sorted alphabetically)
	if len(visible) != 5 {
		t.Errorf("expected 5 visible nodes, got %d", len(visible))
	}

	// Test sibling jumps on Model
	// Let's build a mock model
	m := &Model{
		Root:         root,
		Selected:     nodeA,
		VisibleNodes: visible,
		Cursor:       1, // index of dirA
	}

	// dirA is index 1. Its next sibling should be dirB.
	m.nextSibling()
	if m.Selected.Name != "dirB" {
		t.Errorf("expected next sibling of dirA to be dirB, got %s", m.Selected.Name)
	}

	// dirB is index 4. Its previous sibling should be dirA.
	m.prevSibling()
	if m.Selected.Name != "dirA" {
		t.Errorf("expected prev sibling of dirB to be dirA, got %s", m.Selected.Name)
	}
}
