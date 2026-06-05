# sneak

`sneak` is a high-performance interactive path-picker for Linux terminal pipelines. Built using Go with the Bubble Tea runtime and Lipgloss layout engine, it enables rapid directory traversal and selection matching.

---

## Core Capabilities

* **Pipeline-Compatible Output Routing:** The interactive UI renders exclusively on `stderr`, leaving `stdout` clean to output the picked path. This allows immediate consumption by downstream command substitutions like `cd $(sneak)`.
* **Tree-Semantic Navigation:** Lazily traverses filesystem structures using standard Vim motion controls, keeping disk access overhead at O(1) until directories are expanded.
* **Non-recursive Sibling Jumps:** Capitalized `H`/`L` keys skip deeply nested folder hierarchies entirely, jumping directly to preceding or succeeding siblings.
* **Shallow Fuzzy Filtering:** Pressing `/` limits character-based fuzzy matching strictly to the active folder level, bypassing recursive disk walks and ensuring predictable lookup latency.
* **Zoxide Hub Integration:** Tapping `z` opens a hot-path query list loaded directly from `zoxide query -l`.

---

## Keyboard Layout and Motions

| Key | Mode / Context | Action |
| :--- | :--- | :--- |
| `j` / `Down` | Navigation | Select next visible directory |
| `k` / `Up` | Navigation | Select previous visible directory |
| `h` | Navigation | Collapse active node OR focus parent node (traverses parent directory if at root) |
| `l` | Navigation | Expand active node OR step into first child directory |
| `H` | Navigation | Leap directly to previous sibling directory |
| `L` | Navigation | Leap directly to next sibling directory |
| `/` | Navigation | Activate local fuzzy filter |
| `z` | Navigation | Toggle Zoxide Jump overlay panel |
| `Enter` | Navigation | Confirm selection, print absolute path to `stdout`, and exit |
| `q` / `Esc` | Navigation | Quit program and abort selection |
| `Enter` | Filtering / Modal | Commit filter and return to tree navigation |
| `Esc` | Filtering / Modal | Dismiss search/overlay panel |

---

## Installation and Compilation

Requirements: Go toolchain (1.20+).

```bash
# Clone the repository
cd sneak

# Build the release binary
go build -o sneak

# Run tests
go test -v ./...
```

To install the executable system-wide:
```bash
mv sneak ~/.local/bin/
```

---

## Pipeline Examples

Since `sneak` routes path selection to stdout, it integrates seamlessly with common file operations:

### Fast Navigation
```bash
alias cs='cd $(sneak)'
```

### File Relocation
```bash
# Copy files into a picked directory
cp invoice.pdf $(sneak)

# Move archives
mv archive.tar.gz $(sneak)
```

---

## Architecture Design

For a detailed analysis of the MVU loop, viewport pagination, and state mapping, refer to the [Architecture Guide](file:///home/ayush583/.gemini/antigravity-cli/brain/2a541b85-71ce-402f-8c09-a03b9516c3c0/sneak_architecture.md).
