package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// getZoxidePaths runs `zoxide query -l` to fetch the user's hot-paths list.
func getZoxidePaths() []string {
	cmd := exec.Command("zoxide", "query", "-l")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// If zoxide is not installed or errors, return an empty list gracefully
		return []string{}
	}

	lines := strings.Split(stdout.String(), "\n")
	var paths []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	return paths
}

func main() {
	var startDir string
	if len(os.Args) > 1 {
		startDir = os.Args[1]
	} else {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			startDir = "."
		}
	}

	absStartDir, err := filepath.Abs(startDir)
	if err != nil {
		absStartDir = startDir
	}

	// Fetch zoxide paths for jump capabilities
	zoxidePaths := getZoxidePaths()

	// Initialize the Bubble Tea model
	m := NewModel(absStartDir, zoxidePaths)

	// Run the TUI on stderr so stdout can be clean for downstream commands.
	// We use the Alternate Screen buffer to ensure the terminal state is fully restored on exit.
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running sneak: %v\n", err)
		os.Exit(1)
	}

	// Check if selection was made
	if fm, ok := finalModel.(*Model); ok && fm.FinalPath != "" {
		// Print only the final absolute path to stdout
		fmt.Println(fm.FinalPath)
		os.Exit(0)
	}

	// Exit with 130 (SIGINT standard for shell cancellation) or 1 if aborted
	os.Exit(130)
}
