package main

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
		{"Videos", "do", true},
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

	// Verify TreeConnector characters
	// Find nodeB and nodeA2
	var nodeB *Node
	for _, child := range root.Children {
		if child.Name == "dirB" {
			nodeB = child
			break
		}
	}
	var nodeA2 *Node
	for _, child := range nodeA.Children {
		if child.Name == "dirA2" {
			nodeA2 = child
			break
		}
	}

	if nodeB == nil || nodeA2 == nil {
		t.Fatal("dirB or dirA2 not found in children")
	}

	if nodeA.IsLastChild() {
		t.Errorf("expected dirA to not be the last child")
	}
	if nodeA.TreeConnector(root) != "├── " {
		t.Errorf("expected dirA connector to be '├── ', got %q", nodeA.TreeConnector(root))
	}

	if !nodeB.IsLastChild() {
		t.Errorf("expected dirB to be the last child")
	}
	if nodeB.TreeConnector(root) != "└── " {
		t.Errorf("expected dirB connector to be '└── ', got %q", nodeB.TreeConnector(root))
	}

	if !nodeA2.IsLastChild() {
		t.Errorf("expected dirA2 to be the last child of dirA")
	}
	if nodeA2.TreeConnector(root) != "│   └── " {
		t.Errorf("expected dirA2 connector to be '│   └── ', got %q", nodeA2.TreeConnector(root))
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

func TestAddZoxidePath(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "sneak-zoxide-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temp file inside it
	tmpFile, err := os.CreateTemp(tmpDir, "some-file-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// 1. Test with a valid directory - should run without panic
	addZoxidePath(tmpDir)

	// 2. Test with a file - should return early (no panic)
	addZoxidePath(tmpFile.Name())

	// 3. Test with a non-existent path - should return early (no panic)
	addZoxidePath(filepath.Join(tmpDir, "nonexistent"))
}

func TestUpdateSearchScrolling(t *testing.T) {
	// Setup mock model with visible nodes
	nodeA := &Node{Name: "dirA", Path: "/dirA"}
	nodeB := &Node{Name: "dirB", Path: "/dirB"}
	nodeC := &Node{Name: "dirC", Path: "/dirC"}
	visible := []*Node{nodeA, nodeB, nodeC}

	m := &Model{
		VisibleNodes: visible,
		Cursor:       0,
		Selected:     nodeA,
		SearchActive: true,
		SearchQuery:  "dir",
	}

	// 1. Press Down (KeyDown)
	_, _ = m.updateSearch(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor != 1 {
		t.Errorf("expected Cursor = 1 after KeyDown, got %d", m.Cursor)
	}
	if m.Selected != nodeB {
		t.Errorf("expected Selected = nodeB after KeyDown, got %v", m.Selected)
	}

	// 2. Press Ctrl+J (KeyCtrlJ)
	_, _ = m.updateSearch(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if m.Cursor != 2 {
		t.Errorf("expected Cursor = 2 after KeyCtrlJ, got %d", m.Cursor)
	}
	if m.Selected != nodeC {
		t.Errorf("expected Selected = nodeC after KeyCtrlJ, got %v", m.Selected)
	}

	// 3. Press Up (KeyUp)
	_, _ = m.updateSearch(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor != 1 {
		t.Errorf("expected Cursor = 1 after KeyUp, got %d", m.Cursor)
	}
	if m.Selected != nodeB {
		t.Errorf("expected Selected = nodeB after KeyUp, got %v", m.Selected)
	}

	// 4. Press Ctrl+K (KeyCtrlK)
	_, _ = m.updateSearch(tea.KeyMsg{Type: tea.KeyCtrlK})
	if m.Cursor != 0 {
		t.Errorf("expected Cursor = 0 after KeyCtrlK, got %d", m.Cursor)
	}
	if m.Selected != nodeA {
		t.Errorf("expected Selected = nodeA after KeyCtrlK, got %v", m.Selected)
	}
}

func TestUpdateSearchEnterExpansion(t *testing.T) {
	// Create temp dir structure:
	// tmpDir/
	//   Documents/
	//     work/
	tmpDir, err := os.MkdirTemp("", "sneak-search-enter-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	docs := filepath.Join(tmpDir, "Documents")
	work := filepath.Join(docs, "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	root := NewNode(tmpDir, nil)
	_ = root.LoadChildren()
	root.Expanded = true

	var docsNode *Node
	for _, child := range root.Children {
		if child.Name == "Documents" {
			docsNode = child
			break
		}
	}
	if docsNode == nil {
		t.Fatal("Documents node not found")
	}

	m := &Model{
		Root:         root,
		VisibleNodes: []*Node{root, docsNode},
		Cursor:       1,
		Selected:     docsNode,
		SearchActive: true,
		SearchQuery:  "do",
	}

	// Commit search (KeyEnter)
	_, _ = m.updateSearch(tea.KeyMsg{Type: tea.KeyEnter})

	if m.SearchActive {
		t.Error("expected SearchActive to be false after KeyEnter")
	}
	if !docsNode.Expanded {
		t.Error("expected Documents node to be expanded")
	}
	if len(docsNode.Children) != 1 || docsNode.Children[0].Name != "work" {
		t.Errorf("expected Documents children to be loaded, got %v", docsNode.Children)
	}
	// Verify that visible nodes now contain the child of Documents ("work")
	var workFound bool
	for _, node := range m.VisibleNodes {
		if node.Name == "work" {
			workFound = true
			break
		}
	}
	if !workFound {
		t.Error("expected Documents/work to be visible in tree after expansion")
	}
	// Verify cursor is on Documents node
	if m.Selected != docsNode {
		t.Errorf("expected Selected to remain docsNode, got %v", m.Selected)
	}
}

func TestGoUpAndSearch(t *testing.T) {
	// Create home directory structure:
	// tmpDir/ (home)
	//   Documents/
	//   Downloads/
	//   github/
	//     SneakDirectory/
	tmpDir, err := os.MkdirTemp("", "sneak-home-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	docs := filepath.Join(tmpDir, "Documents")
	downloads := filepath.Join(tmpDir, "Downloads")
	github := filepath.Join(tmpDir, "github")
	sneak := filepath.Join(github, "SneakDirectory")

	for _, d := range []string{docs, downloads, github, sneak} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	// Start at deep directory
	m := NewModel(sneak, nil)

	// Go up once (to github)
	m.goUpDirectory()
	if m.Root.Path != github {
		t.Errorf("expected Root to be github (%q), got %q", github, m.Root.Path)
	}

	// Go up twice (to home)
	m.goUpDirectory()
	if m.Root.Path != tmpDir {
		t.Errorf("expected Root to be home (%q), got %q", tmpDir, m.Root.Path)
	}

	// Verify home's children are loaded
	if len(m.Root.Children) < 3 {
		t.Errorf("expected at least 3 children, got %d", len(m.Root.Children))
	}

	// Trigger search for "do"
	m.SearchActive = true
	m.SearchQuery = "do"
	m.updateVisibleNodes()

	// We expect both Documents and Downloads in visible nodes
	var foundDocs, foundDownloads bool
	var names []string
	for _, node := range m.VisibleNodes {
		names = append(names, node.Name)
		if node.Name == "Documents" {
			foundDocs = true
		}
		if node.Name == "Downloads" {
			foundDownloads = true
		}
	}

	if !foundDocs {
		t.Errorf("expected Documents in search results, visible nodes: %v", names)
	}
	if !foundDownloads {
		t.Errorf("expected Downloads in search results, visible nodes: %v", names)
	}
}

func TestVimTopBottomKeybindings(t *testing.T) {
	nodeA := &Node{Name: "dirA", Path: "/dirA"}
	nodeB := &Node{Name: "dirB", Path: "/dirB"}
	nodeC := &Node{Name: "dirC", Path: "/dirC"}
	visible := []*Node{nodeA, nodeB, nodeC}

	m := &Model{
		VisibleNodes: visible,
		Cursor:       1,
		Selected:     nodeB,
	}

	// 1. Press "G" - should jump to bottom (nodeC)
	_, _ = m.updateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.Cursor != 2 {
		t.Errorf("expected Cursor = 2 after G, got %d", m.Cursor)
	}
	if m.Selected != nodeC {
		t.Errorf("expected Selected = nodeC after G, got %v", m.Selected)
	}

	// 2. Press "g" - should set gPending = true
	_, _ = m.updateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if !m.gPending {
		t.Error("expected gPending to be true after pressing 'g'")
	}

	// 3. Press "g" again - should jump to top (nodeA) and clear gPending
	_, _ = m.updateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.gPending {
		t.Error("expected gPending to be false after second 'g'")
	}
	if m.Cursor != 0 {
		t.Errorf("expected Cursor = 0 after gg, got %d", m.Cursor)
	}
	if m.Selected != nodeA {
		t.Errorf("expected Selected = nodeA after gg, got %v", m.Selected)
	}

	// 4. Press "g" then "j" - should clear gPending and execute "j"
	m.Cursor = 0
	m.Selected = nodeA
	_, _ = m.updateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	_, _ = m.updateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.gPending {
		t.Error("expected gPending to be false after pressing 'g' then 'j'")
	}
	if m.Cursor != 1 {
		t.Errorf("expected Cursor = 1 after g then j, got %d", m.Cursor)
	}
	if m.Selected != nodeB {
		t.Errorf("expected Selected = nodeB after g then j, got %v", m.Selected)
	}
}

func TestCenterCursor(t *testing.T) {
	nodeA := &Node{Name: "dirA"}
	nodeB := &Node{Name: "dirB"}
	nodeC := &Node{Name: "dirC"}
	nodeD := &Node{Name: "dirD"}
	nodeE := &Node{Name: "dirE"}
	visible := []*Node{nodeA, nodeB, nodeC, nodeD, nodeE}

	m := &Model{
		VisibleNodes: visible,
		Cursor:       2, // center nodeC
	}

	// 1. Viewport fits all nodes: ScrollOffset should be 0
	m.centerCursor(10)
	if m.ScrollOffset != 0 {
		t.Errorf("expected ScrollOffset = 0 when viewport fits all nodes, got %d", m.ScrollOffset)
	}

	// 2. Viewport height 3: center nodeC (Cursor=2)
	// target ScrollOffset = 2 - 3/2 = 2 - 1 = 1
	m.centerCursor(3)
	if m.ScrollOffset != 1 {
		t.Errorf("expected ScrollOffset = 1, got %d", m.ScrollOffset)
	}

	// 3. Cursor at bottom (Cursor=4), viewport height 3
	// target ScrollOffset = 4 - 3/2 = 4 - 1 = 3 -> clamped to len(visible)-maxLines = 5-3 = 2
	m.Cursor = 4
	m.centerCursor(3)
	if m.ScrollOffset != 2 {
		t.Errorf("expected ScrollOffset = 2 (clamped), got %d", m.ScrollOffset)
	}

	// 4. Cursor at top (Cursor=0), viewport height 3
	// target ScrollOffset = 0 - 3/2 = 0 - 1 = -1 -> clamped to 0
	m.Cursor = 0
	m.centerCursor(3)
	if m.ScrollOffset != 0 {
		t.Errorf("expected ScrollOffset = 0 (clamped), got %d", m.ScrollOffset)
	}
}
