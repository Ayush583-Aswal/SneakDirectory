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

// addZoxidePath runs `zoxide add <path>` to record the directory in the user's database.
func addZoxidePath(path string) {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return
	}
	cmd := exec.Command("zoxide", "add", path)
	_ = cmd.Run()
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
		// Add selected directory to zoxide
		addZoxidePath(fm.FinalPath)

		// Print the substituted command to stderr so it shows up above the prompt
		printProcessedCommand(fm.FinalPath)
		// Print only the final absolute path to stdout
		fmt.Println(fm.FinalPath)
		os.Exit(0)
	}

	// Exit with 130 (SIGINT standard for shell cancellation) or 1 if aborted
	os.Exit(130)
}

// getParentPID returns the PPID of the given pid by reading /proc/<pid>/stat.
func getParentPID(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	str := string(data)
	lastParen := strings.LastIndex(str, ")")
	if lastParen == -1 || lastParen+2 >= len(str) {
		return 0, fmt.Errorf("invalid stat format")
	}
	fields := strings.Split(str[lastParen+2:], " ")
	if len(fields) < 2 {
		return 0, fmt.Errorf("invalid stat fields")
	}
	var ppid int
	_, err = fmt.Sscanf(fields[1], "%d", &ppid)
	return ppid, err
}

// getParentCommandLine walks up the PPID chain and returns the first command line containing "sneak".
func getParentCommandLine() string {
	pid := os.Getppid()
	for i := 0; i < 5; i++ { // limit search to 5 ancestors
		if pid <= 1 {
			break
		}
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err == nil {
			// cmdline arguments are null-separated
			parts := strings.Split(string(data), "\x00")
			var args []string
			for _, part := range parts {
				if part != "" {
					args = append(args, part)
				}
			}
			if len(args) > 0 {
				fullCmd := strings.Join(args, " ")
				if strings.Contains(fullCmd, "sneak") {
					return fullCmd
				}
			}
		}
		// Go to parent PID
		parentPid, err := getParentPID(pid)
		if err != nil || parentPid == pid {
			break
		}
		pid = parentPid
	}
	return ""
}

// replaceSneakInvocation replaces the call to sneak with selectedPath.
func replaceSneakInvocation(cmd, selectedPath string) string {
	// 1. Check for command substitution $(...) containing sneak
	startIdx := strings.Index(cmd, "$(")
	if startIdx != -1 {
		endIdx := strings.Index(cmd[startIdx:], ")")
		if endIdx != -1 {
			sub := cmd[startIdx : startIdx+endIdx+1]
			if strings.Contains(sub, "sneak") {
				return cmd[:startIdx] + selectedPath + cmd[startIdx+endIdx+1:]
			}
		}
	}

	// 2. Check for backticks `...` containing sneak
	btStart := strings.Index(cmd, "`")
	if btStart != -1 {
		btEnd := strings.Index(cmd[btStart+1:], "`")
		if btEnd != -1 {
			sub := cmd[btStart : btStart+btEnd+2]
			if strings.Contains(sub, "sneak") {
				return cmd[:btStart] + selectedPath + cmd[btStart+btEnd+2:]
			}
		}
	}

	// 3. Fallback: replace "sneak" name directly
	if strings.Contains(cmd, "sneak") {
		return strings.Replace(cmd, "sneak", selectedPath, 1)
	}

	return cmd
}

// printProcessedCommand resolves the calling command and prints it to stderr.
func printProcessedCommand(selectedPath string) {
	cmd := getParentCommandLine()
	if cmd != "" {
		processed := replaceSneakInvocation(cmd, selectedPath)
		processed = strings.TrimSpace(processed)
		fmt.Fprintf(os.Stderr, "[sneak] processed command: %s\n", processed)
	} else {
		// Fallback if we cannot read the command (e.g. on pure interactive TTY PPID where /proc/$PPID/cmdline is just "bash")
		fmt.Fprintf(os.Stderr, "[sneak] selected path: %s\n", selectedPath)
	}
}
