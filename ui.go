package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styling constants using a vibrant Catppuccin-inspired dark theme.
var (
	// Custom renderer bound to stderr so that color detection is based on the TUI output stream
	// instead of stdout (which is redirected during pipeline execution).
	Renderer = lipgloss.NewRenderer(os.Stderr)

	PrimaryColor   = lipgloss.Color("#7D56F4") // Vibrant Violet
	AccentColor    = lipgloss.Color("#04B575") // Forest Green
	WarningColor   = lipgloss.Color("#FF5F87") // Coral/Pink
	BgColor        = lipgloss.Color("#1E1E2E") // Dark Charcoal
	FgColor        = lipgloss.Color("#CDD6F4") // Soft Text
	SubtleColor    = lipgloss.Color("#6C7086") // Slate Grey
	SelectionColor = lipgloss.Color("#313244") // Darker Highlight Grey

	// FZF highlighting colors
	FzfMatchColor         = lipgloss.Color("#FAB387") // Warm Orange
	FzfMatchSelectedColor = lipgloss.Color("#F5C2E7") // Soft Rosewater
	GitBranchColor        = lipgloss.Color("#BD93F9") // Starship Git Branch Purple

	TitleStyle = Renderer.NewStyle().
			Background(PrimaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	RootStyle = Renderer.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	FolderStyle = Renderer.NewStyle().
			Foreground(FgColor)

	SelectedFolderStyle = Renderer.NewStyle().
				Background(SelectionColor).
				Foreground(PrimaryColor).
				Bold(true)

	SearchPromptStyle = Renderer.NewStyle().
				Foreground(AccentColor).
				Bold(true)

	SearchInputStyle = Renderer.NewStyle().
				Foreground(FgColor)

	HelpStyle = Renderer.NewStyle().
			Foreground(SubtleColor).
			Italic(true)

	ModalStyle = Renderer.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2).
			Background(BgColor)
)

type Model struct {
	Root         *Node
	Selected     *Node
	VisibleNodes []*Node
	Cursor       int
	ScrollOffset int

	SearchActive bool
	SearchNode   *Node
	SearchQuery  string

	ZoxideActive   bool
	ZoxidePaths    []string
	ZoxideFiltered []string
	ZoxideCursor   int
	ZoxideScroll   int
	ZoxideQuery    string

	Width  int
	Height int

	FinalPath string
	Quitting  bool
	GitBranch string // Cached Git branch name of active root
}

func NewModel(rootPath string, zoxidePaths []string) *Model {
	root := NewNode(rootPath, nil)
	_ = root.LoadChildren()
	root.Expanded = true

	m := &Model{
		Root:         root,
		Selected:     root,
		ZoxidePaths:  zoxidePaths,
		ZoxideActive: false,
	}
	m.updateGitBranch()
	m.updateVisibleNodes()
	return m
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) keepCursorInView(maxLines int) {
	if m.Cursor < m.ScrollOffset {
		m.ScrollOffset = m.Cursor
	}
	if m.Cursor >= m.ScrollOffset+maxLines {
		m.ScrollOffset = m.Cursor - maxLines + 1
	}
	if m.ScrollOffset < 0 {
		m.ScrollOffset = 0
	}
}

func (m *Model) goUpDirectory() {
	parentDir := filepath.Dir(m.Root.Path)
	if parentDir == m.Root.Path {
		return // Already at filesystem root
	}
	oldRootPath := m.Root.Path

	newRoot := NewNode(parentDir, nil)
	_ = newRoot.LoadChildren()
	newRoot.Expanded = true

	m.Root = newRoot
	m.Selected = newRoot

	m.updateGitBranch()
	m.updateVisibleNodes()

	// Place cursor on the folder we just exited
	for i, n := range m.VisibleNodes {
		if n.Path == oldRootPath {
			m.Selected = n
			m.Cursor = i
			break
		}
	}
	m.keepCursorInView(m.Height - 5)
}

func getGitBranch(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

func (m *Model) updateGitBranch() {
	m.GitBranch = getGitBranch(m.Root.Path)
}

func (m *Model) nextSibling() {
	if m.Selected.Parent == nil {
		return
	}
	for i := m.Cursor + 1; i < len(m.VisibleNodes); i++ {
		if m.VisibleNodes[i].Parent == m.Selected.Parent {
			m.Selected = m.VisibleNodes[i]
			m.Cursor = i
			break
		}
	}
}

func (m *Model) prevSibling() {
	if m.Selected.Parent == nil {
		return
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		if m.VisibleNodes[i].Parent == m.Selected.Parent {
			m.Selected = m.VisibleNodes[i]
			m.Cursor = i
			break
		}
	}
}

func (m *Model) updateVisibleNodes() {
	allElaborated := m.Root.GetVisibleNodes(nil, "")

	if m.SearchActive && m.SearchQuery != "" {
		var filtered []*Node
		for _, node := range allElaborated {
			if matched, _ := fuzzyMatchWithIndices(node.Name, m.SearchQuery); matched {
				filtered = append(filtered, node)
			}
		}
		m.VisibleNodes = filtered
	} else {
		m.VisibleNodes = allElaborated
	}

	found := false
	for i, n := range m.VisibleNodes {
		if n == m.Selected {
			m.Cursor = i
			found = true
			break
		}
	}

	if !found {
		if len(m.VisibleNodes) > 0 {
			if m.Cursor >= len(m.VisibleNodes) {
				m.Cursor = len(m.VisibleNodes) - 1
			}
			if m.Cursor < 0 {
				m.Cursor = 0
			}
			m.Selected = m.VisibleNodes[m.Cursor]
		} else {
			m.Cursor = 0
			m.Selected = m.Root
		}
	}
}

func (m *Model) filterZoxide() {
	if m.ZoxideQuery == "" {
		m.ZoxideFiltered = m.ZoxidePaths
	} else {
		var filtered []string
		for _, path := range m.ZoxidePaths {
			if fuzzyMatch(path, m.ZoxideQuery) {
				filtered = append(filtered, path)
			}
		}
		m.ZoxideFiltered = filtered
	}
	m.ZoxideCursor = 0
	m.ZoxideScroll = 0
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Universal abort hook
		if msg.Type == tea.KeyCtrlC {
			m.Quitting = true
			return m, tea.Quit
		}

		if m.ZoxideActive {
			return m.updateZoxide(msg)
		} else if m.SearchActive {
			return m.updateSearch(msg)
		} else {
			return m.updateTree(msg)
		}
	}
	return m, nil
}

func (m *Model) updateTree(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.Quitting = true
		return m, tea.Quit

	case "enter":
		m.FinalPath = m.Selected.Path
		m.Quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.Cursor < len(m.VisibleNodes)-1 {
			m.Cursor++
			m.Selected = m.VisibleNodes[m.Cursor]
		}

	case "k", "up":
		if m.Cursor > 0 {
			m.Cursor--
			m.Selected = m.VisibleNodes[m.Cursor]
		}

	case "h", "left":
		if m.Selected.Expanded {
			m.Selected.Expanded = false
			m.updateVisibleNodes()
		} else if m.Selected.Parent != nil {
			m.Selected = m.Selected.Parent
			m.updateVisibleNodes()
		} else {
			m.goUpDirectory()
		}

	case "l", "right":
		if !m.Selected.Expanded {
			m.Selected.Expanded = true
			_ = m.Selected.LoadChildren()
			m.updateVisibleNodes()
		} else if len(m.Selected.Children) > 0 {
			var firstChild *Node
			for _, child := range m.Selected.Children {
				for _, vis := range m.VisibleNodes {
					if vis == child {
						firstChild = child
						break
					}
				}
				if firstChild != nil {
					break
				}
			}
			if firstChild != nil {
				m.Selected = firstChild
				m.updateVisibleNodes()
			} else {
				m.Selected = m.Selected.Children[0]
				m.updateVisibleNodes()
			}
		}

	case "L":
		m.nextSibling()

	case "H":
		m.prevSibling()

	case "/":
		m.SearchActive = true
		m.SearchQuery = ""
		m.updateVisibleNodes()

	case "z":
		m.ZoxideActive = true
		m.ZoxideQuery = ""
		m.ZoxideFiltered = m.ZoxidePaths
		m.ZoxideCursor = 0
		m.ZoxideScroll = 0
	}

	return m, nil
}

func (m *Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.SearchActive = false
		return m, nil

	case tea.KeyEsc:
		m.SearchActive = false
		m.SearchQuery = ""
		m.updateVisibleNodes()
		return m, nil

	case tea.KeyBackspace:
		if len(m.SearchQuery) > 0 {
			m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
			m.updateVisibleNodes()
		}

	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.SearchQuery += msg.String()
			m.updateVisibleNodes()
		}
	}
	return m, nil
}

func (m *Model) updateZoxide(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.ZoxideActive = false
		return m, nil

	case tea.KeyEnter:
		if len(m.ZoxideFiltered) > 0 {
			selectedPath := m.ZoxideFiltered[m.ZoxideCursor]
			m.Root = NewNode(selectedPath, nil)
			_ = m.Root.LoadChildren()
			m.Root.Expanded = true
			m.Selected = m.Root
			m.ZoxideActive = false
			m.updateGitBranch()
			m.updateVisibleNodes()
		}
		return m, nil

	case tea.KeyUp, tea.KeyCtrlK:
		if m.ZoxideCursor > 0 {
			m.ZoxideCursor--
		}

	case tea.KeyDown, tea.KeyCtrlJ:
		if m.ZoxideCursor < len(m.ZoxideFiltered)-1 {
			m.ZoxideCursor++
		}

	case tea.KeyBackspace:
		if len(m.ZoxideQuery) > 0 {
			m.ZoxideQuery = m.ZoxideQuery[:len(m.ZoxideQuery)-1]
			m.filterZoxide()
		}

	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.ZoxideQuery += msg.String()
			m.filterZoxide()
		}
	}
	return m, nil
}

func (m *Model) renderHeader() string {
	var s strings.Builder

	displayPath := m.Root.Path
	homeDir := os.Getenv("HOME")
	if homeDir != "" && strings.HasPrefix(displayPath, homeDir) {
		displayPath = "~" + strings.TrimPrefix(displayPath, homeDir)
	}

	pathStyle := Renderer.NewStyle().Foreground(PrimaryColor).Bold(true)
	icon := ""
	if m.Root.Path == homeDir {
		icon = ""
	} else if m.Root.Path == "/" {
		icon = ""
	}

	s.WriteString(" " + pathStyle.Render(icon+" "+displayPath))

	if m.GitBranch != "" {
		gitStyle := Renderer.NewStyle().Foreground(GitBranchColor)
		s.WriteString(gitStyle.Render(" on  "+m.GitBranch))
	}

	if m.SearchActive {
		searchStyle := Renderer.NewStyle().Foreground(WarningColor).Bold(true)
		s.WriteString(searchStyle.Render(" [search: "+m.SearchQuery+"]"))
	}

	headerLine := s.String()
	divider := Renderer.NewStyle().Foreground(SubtleColor).Render(strings.Repeat("─", m.Width))
	return headerLine + "\n" + divider
}

func (m *Model) renderTree(maxLines int) string {
	if len(m.VisibleNodes) == 0 {
		return "\n  " + Renderer.NewStyle().Foreground(SubtleColor).Render("(No matching directories found)") + strings.Repeat("\n", maxLines-1)
	}

	var lines []string
	start := m.ScrollOffset
	end := start + maxLines
	if end > len(m.VisibleNodes) {
		end = len(m.VisibleNodes)
	}

	for i := start; i < end; i++ {
		node := m.VisibleNodes[i]
		depth := node.Depth(m.Root)
		indent := strings.Repeat("  ", depth)

		prefix := "  "
		if node.Expanded {
			prefix = "  "
		}

		nodeName := node.Name
		if node == m.Root {
			nodeName = " ~ " + filepath.Base(node.Path)
			prefix = " "
		}

		var matchIndices []int
		if m.SearchActive && m.SearchQuery != "" {
			_, matchIndices = fuzzyMatchWithIndices(nodeName, m.SearchQuery)
		}

		var baseStyle lipgloss.Style
		if i == m.Cursor {
			baseStyle = SelectedFolderStyle
		} else {
			baseStyle = FolderStyle
		}

		var highlightedName string
		if len(matchIndices) > 0 {
			var highlightStyle lipgloss.Style
			if i == m.Cursor {
				highlightStyle = Renderer.NewStyle().Foreground(FzfMatchSelectedColor).Bold(true).Underline(true)
			} else {
				highlightStyle = Renderer.NewStyle().Foreground(FzfMatchColor).Bold(true)
			}
			highlightedName = highlightText(nodeName, matchIndices, baseStyle, highlightStyle)
		} else {
			highlightedName = baseStyle.Render(nodeName)
		}

		var lineText string
		indentStyle := Renderer.NewStyle().Foreground(SubtleColor)
		if i == m.Cursor {
			lineText = indentStyle.Render("❯ "+indent) + baseStyle.Render(prefix) + highlightedName
		} else {
			lineText = indentStyle.Render("  "+indent) + baseStyle.Render(prefix) + highlightedName
		}

		lines = append(lines, lineText)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m *Model) renderFooter() string {
	var s strings.Builder
	divider := Renderer.NewStyle().Foreground(SubtleColor).Render(strings.Repeat("─", m.Width))
	s.WriteString(divider + "\n")

	if m.SearchActive {
		prompt := SearchPromptStyle.Render("  search ❯ ")
		input := SearchInputStyle.Render(m.SearchQuery)
		s.WriteString(prompt + input + "█\n")
		s.WriteString(HelpStyle.Render("  esc: cancel search • enter: lock query • j/k: navigate"))
	} else {
		s.WriteString(HelpStyle.Render("  j/k: navigate • h/l: up/down tree • H/L: sibling leap • /: search • z: zoxide • enter: pick • q: quit"))
	}

	return s.String()
}

func (m *Model) renderZoxideModal() string {
	var s strings.Builder

	s.WriteString(Renderer.NewStyle().Foreground(PrimaryColor).Bold(true).Render("ZOXIDE JUMP PANEL") + "\n\n")

	prompt := Renderer.NewStyle().Foreground(AccentColor).Bold(true).Render("Filter paths: ")
	s.WriteString(prompt + m.ZoxideQuery + "█\n\n")

	modalWidth := m.Width - 10
	if modalWidth > 90 {
		modalWidth = 90
	}
	if modalWidth < 30 {
		modalWidth = 30
	}

	modalHeight := m.Height - 6
	if modalHeight > 22 {
		modalHeight = 22
	}
	if modalHeight < 7 {
		modalHeight = 7
	}

	maxListLines := modalHeight - 7
	if maxListLines <= 0 {
		maxListLines = 1
	}

	if m.ZoxideCursor < m.ZoxideScroll {
		m.ZoxideScroll = m.ZoxideCursor
	}
	if m.ZoxideCursor >= m.ZoxideScroll+maxListLines {
		m.ZoxideScroll = m.ZoxideCursor - maxListLines + 1
	}
	if m.ZoxideScroll < 0 {
		m.ZoxideScroll = 0
	}

	if len(m.ZoxideFiltered) == 0 {
		s.WriteString(Renderer.NewStyle().Foreground(SubtleColor).Render("  No matching zoxide paths found\n"))
		for i := 1; i < maxListLines; i++ {
			s.WriteString("\n")
		}
	} else {
		end := m.ZoxideScroll + maxListLines
		if end > len(m.ZoxideFiltered) {
			end = len(m.ZoxideFiltered)
		}

		for i := m.ZoxideScroll; i < end; i++ {
			path := m.ZoxideFiltered[i]
			displayPath := path
			if strings.HasPrefix(path, os.Getenv("HOME")) {
				displayPath = "~" + strings.TrimPrefix(path, os.Getenv("HOME"))
			}

			if i == m.ZoxideCursor {
				s.WriteString(SelectedFolderStyle.Render(" ❯ "+displayPath) + "\n")
			} else {
				s.WriteString(Renderer.NewStyle().Foreground(FgColor).Render("   "+displayPath) + "\n")
			}
		}

		for i := end - m.ZoxideScroll; i < maxListLines; i++ {
			s.WriteString("\n")
		}
	}

	s.WriteString("\n" + HelpStyle.Render("enter: select • esc: close panel • ctrl+j/k: navigate"))

	modalStyled := ModalStyle.Width(modalWidth).Height(modalHeight).Render(s.String())
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalStyled)
}

func (m Model) View() string {
	if m.Quitting {
		return ""
	}

	// Calculate heights
	headerHeight := 3
	footerHeight := 2
	if m.SearchActive {
		footerHeight = 3
	}
	maxTreeLines := m.Height - headerHeight - footerHeight
	if maxTreeLines <= 0 {
		maxTreeLines = 1
	}
	m.keepCursorInView(maxTreeLines)

	if m.ZoxideActive {
		return m.renderZoxideModal()
	}

	var s strings.Builder
	s.WriteString(m.renderHeader() + "\n")
	s.WriteString(m.renderTree(maxTreeLines) + "\n")
	s.WriteString(m.renderFooter())

	return s.String()
}

func highlightText(text string, indices []int, baseStyle, highlightStyle lipgloss.Style) string {
	if len(indices) == 0 {
		return baseStyle.Render(text)
	}
	runes := []rune(text)
	var sb strings.Builder
	idxMap := make(map[int]bool)
	for _, idx := range indices {
		idxMap[idx] = true
	}

	for i, r := range runes {
		charStr := string(r)
		if idxMap[i] {
			sb.WriteString(highlightStyle.Render(charStr))
		} else {
			sb.WriteString(baseStyle.Render(charStr))
		}
	}
	return sb.String()
}
