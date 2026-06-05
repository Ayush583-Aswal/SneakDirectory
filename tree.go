package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// fuzzyMatch checks if the query is a subsequence of the target.
func fuzzyMatch(target, query string) bool {
	if query == "" {
		return true
	}
	target = strings.ToLower(target)
	query = strings.ToLower(query)

	tIdx, qIdx := 0, 0
	for tIdx < len(target) && qIdx < len(query) {
		if target[tIdx] == query[qIdx] {
			qIdx++
		}
		tIdx++
	}
	return qIdx == len(query)
}
