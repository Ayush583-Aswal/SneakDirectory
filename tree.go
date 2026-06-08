package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Node represents a directory in the filesystem tree.
type Node struct {
	Path     string
	Name     string
	IsDir    bool
	Parent   *Node
	Children []*Node
	Expanded bool
	Loaded   bool
}

// NewNode creates a new Node instance.
func NewNode(path string, parent *Node) *Node {
	name := filepath.Base(path)
	if name == "" || name == "." || name == "/" {
		name = path
	}
	return &Node{
		Path:   path,
		Name:   name,
		IsDir:  true,
		Parent: parent,
	}
}

// LoadChildren reads the directory contents and populates the child nodes.
// It skips files and only loads directories to optimize performance.
func (n *Node) LoadChildren() error {
	if n.Loaded {
		return nil
	}

	entries, err := os.ReadDir(n.Path)
	if err != nil {
		// If we can't read the directory (e.g. permission denied), keep it empty
		n.Loaded = true
		return err
	}

	n.Children = nil
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			// Exclude dot directories like "." and ".." but include hidden folders like ".config"
			if name == "." || name == ".." {
				continue
			}
			childPath := filepath.Join(n.Path, name)
			n.Children = append(n.Children, &Node{
				Path:   childPath,
				Name:   name,
				IsDir:  true,
				Parent: n,
			})
		}
	}

	// Sort children alphabetically (case-insensitive)
	sort.Slice(n.Children, func(i, j int) bool {
		return strings.ToLower(n.Children[i].Name) < strings.ToLower(n.Children[j].Name)
	})

	n.Loaded = true
	return nil
}

// GetVisibleNodes flattens the visible tree nodes recursively.
// If n is the searchNode, its immediate children are filtered using the fuzzy query.
func (n *Node) GetVisibleNodes(searchNode *Node, searchQuery string) []*Node {
	var list []*Node
	n.flatten(&list, searchNode, searchQuery)
	return list
}

func (n *Node) flatten(list *[]*Node, searchNode *Node, searchQuery string) {
	*list = append(*list, n)
	if n.Expanded {
		// Load children on the fly if not loaded yet
		if !n.Loaded {
			_ = n.LoadChildren()
		}

		var children []*Node
		if n == searchNode && searchQuery != "" {
			for _, child := range n.Children {
				if fuzzyMatch(child.Name, searchQuery) {
					children = append(children, child)
				}
			}
		} else {
			children = n.Children
		}

		for _, child := range children {
			child.flatten(list, searchNode, searchQuery)
		}
	}
}

// Depth calculates the depth of the node relative to the active root.
func (n *Node) Depth(root *Node) int {
	depth := 0
	curr := n
	for curr != nil && curr != root {
		depth++
		curr = curr.Parent
	}
	return depth
}

// IsLastChild checks if the node is the last child of its parent.
func (n *Node) IsLastChild() bool {
	if n.Parent == nil {
		return true
	}
	siblings := n.Parent.Children
	if len(siblings) == 0 {
		return true
	}
	return siblings[len(siblings)-1] == n
}

// TreeConnector generates the tree line prefix (e.g. "│   └── ") for the node.
func (n *Node) TreeConnector(root *Node) string {
	if n == root {
		return ""
	}

	// Walk up to gather ancestors (excluding the node itself and the root)
	var ancestors []*Node
	curr := n.Parent
	for curr != nil && curr != root {
		ancestors = append(ancestors, curr)
		curr = curr.Parent
	}

	var sb strings.Builder
	// Process ancestors from top-down
	for i := len(ancestors) - 1; i >= 0; i-- {
		anc := ancestors[i]
		if anc.IsLastChild() {
			sb.WriteString("    ")
		} else {
			sb.WriteString("│   ")
		}
	}

	if n.IsLastChild() {
		sb.WriteString("└── ")
	} else {
		sb.WriteString("├── ")
	}

	return sb.String()
}

// fuzzyMatch checks if the query is a subsequence of the target.
func fuzzyMatch(target, query string) bool {
	matched, _ := fuzzyMatchWithIndices(target, query)
	return matched
}

// fuzzyMatchWithIndices checks if the query is a subsequence of the target,
// returning the match status and target rune indices that matched.
func fuzzyMatchWithIndices(target, query string) (bool, []int) {
	if query == "" {
		return true, nil
	}
	targetRunes := []rune(target)
	queryRunes := []rune(strings.ToLower(query))

	var indices []int
	tIdx, qIdx := 0, 0
	for tIdx < len(targetRunes) && qIdx < len(queryRunes) {
		rT := unicode.ToLower(targetRunes[tIdx])
		rQ := queryRunes[qIdx]
		if rT == rQ {
			indices = append(indices, tIdx)
			qIdx++
		}
		tIdx++
	}
	if qIdx == len(queryRunes) {
		return true, indices
	}
	return false, nil
}
