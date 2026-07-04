package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- History ---

// HistoryEntry represents a recently used directory for quick-select.
type HistoryEntry struct {
	Path        string    // Full path or ~/... format
	DisplayPath string    // Display path (with ~ expanded)
	LastUsedAt  time.Time // Last used timestamp
}

// DirHistoryRemoveMsg is emitted when a history entry should be removed from persistent storage.
type DirHistoryRemoveMsg struct {
	Path string
}

// DirPickerModel is a directory browser component for selecting a working directory.
type DirPickerModel struct {
	currentDir string   // Currently displayed directory
	entries    []string // Subdirectory names in the current directory
	filtered   []string // Filtered entries
	cursor     int      // Cursor position (on combined history+directory list)
	offset     int      // Scroll offset

	filterInput textinput.Model // Filter input
	showHidden  bool            // Whether to show hidden directories

	selected bool   // Whether a directory was selected
	result   string // Selected directory path

	width  int
	height int

	// History (recently used directories)
	historyDirs     []HistoryEntry // History entries
	filteredHistory []HistoryEntry // Filtered history entries
}

// NewDirPickerModel creates a new directory picker starting at the given path.
// If startDir is empty, defaults to the user's home directory.
func NewDirPickerModel(startDir string) DirPickerModel {
	if startDir == "" {
		startDir, _ = os.UserHomeDir()
	}
	// Expand ~
	if strings.HasPrefix(startDir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			startDir = filepath.Join(home, startDir[2:])
		}
	}
	// Ensure absolute path
	if !filepath.IsAbs(startDir) {
		if abs, err := filepath.Abs(startDir); err == nil {
			startDir = abs
		}
	}

	fi := textinput.New()
	fi.Placeholder = "type to filter..."
	fi.CharLimit = 256
	fi.Width = 40
	fi.Focus()

	m := DirPickerModel{
		currentDir:  startDir,
		filterInput: fi,
	}
	m.loadEntries()
	return m
}

// SetHistory sets the directory history entries for quick-select.
// Entries should be pre-sorted by LastUsedAt (most recent first).
func (m *DirPickerModel) SetHistory(entries []HistoryEntry) {
	m.historyDirs = entries
	m.updateFilteredHistory()
}

// Selected returns true if a directory was selected.
func (m *DirPickerModel) Selected() bool {
	return m.selected
}

// Result returns the selected directory path.
func (m *DirPickerModel) Result() string {
	return m.result
}

// Init implements tea.Model.
func (m DirPickerModel) Init() tea.Cmd {
	return textinput.Blink
}

// totalItems returns the total number of selectable items (history + directories).
func (m DirPickerModel) totalItems() int {
	return len(m.filteredHistory) + len(m.filtered)
}

// Update implements tea.Model.
func (m DirPickerModel) Update(msg tea.Msg) (DirPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		historyLen := len(m.filteredHistory)

		switch msg.String() {
		case "enter":
			if m.totalItems() > 0 && m.cursor < m.totalItems() {
				if m.cursor < historyLen {
					// History entry selected: return the path immediately
					entry := m.filteredHistory[m.cursor]

					checkPath := entry.Path
					if strings.HasPrefix(checkPath, "~/") {
						if home, err := os.UserHomeDir(); err == nil {
							checkPath = filepath.Join(home, checkPath[2:])
						}
					}
					if info, err := os.Stat(checkPath); err != nil || !info.IsDir() {
						// Does not exist: remove from display list + return remove message
						m.removeFilteredHistoryEntry(m.cursor)
						return m, func() tea.Msg {
							return DirHistoryRemoveMsg{Path: entry.Path}
						}
					}

					m.selected = true
					m.result = entry.Path
					return m, nil
				}
				// Enter regular directory
				dirIdx := m.cursor - historyLen
				if dirIdx < len(m.filtered) {
					selected := m.filtered[dirIdx]
					m.currentDir = filepath.Join(m.currentDir, selected)
					m.filterInput.SetValue("")
					m.cursor = 0
					m.offset = 0
					m.loadEntries()
				}
			}
			return m, nil

		case "tab", "ctrl+d":
			// Select current directory
			m.selected = true
			m.result = m.currentDir
			return m, nil

		case "backspace":
			// If filter is empty, go to parent
			if m.filterInput.Value() == "" {
				parent := filepath.Dir(m.currentDir)
				if parent != m.currentDir {
					m.currentDir = parent
					m.cursor = 0
					m.offset = 0
					m.loadEntries()
				}
				return m, nil
			}

		case "up":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
			return m, nil

		case "down":
			if m.cursor < m.totalItems()-1 {
				m.cursor++
				m.adjustScroll()
			}
			return m, nil

		case "left":
			// Jump to top of history section
			if len(m.filteredHistory) > 0 {
				m.cursor = 0
				m.adjustScroll()
			}
			return m, nil

		case "right":
			// Jump to top of directory section
			historyLen := len(m.filteredHistory)
			if len(m.filtered) > 0 {
				m.cursor = historyLen
				m.adjustScroll()
			}
			return m, nil

		case "ctrl+h":
			// Toggle hidden directories
			m.showHidden = !m.showHidden
			m.loadEntries()
			return m, nil
		}
	}

	// Update filter input
	oldQuery := m.filterInput.Value()
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)

	// Check for direct path navigation
	val := m.filterInput.Value()
	if val != oldQuery {
		if strings.HasPrefix(val, "/") || strings.HasPrefix(val, "~") {
			path := val
			if strings.HasPrefix(path, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					path = filepath.Join(home, path[2:])
				}
			}
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				m.currentDir = path
				m.filterInput.SetValue("")
				m.cursor = 0
				m.offset = 0
				m.loadEntries()
				return m, cmd
			}
		}
		// Normal filtering
		m.applyFilter()
		m.updateFilteredHistory()
	}

	return m, cmd
}

// View renders the directory picker.
func (m DirPickerModel) View() string {
	var b strings.Builder

	// Breadcrumb: current path
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	displayPath := m.currentDir
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(displayPath, home) {
		displayPath = "~" + displayPath[len(home):]
	}
	b.WriteString(pathStyle.Render("  📂 " + displayPath))
	b.WriteString("\n")

	// Filter input
	b.WriteString("  " + m.filterInput.View())
	b.WriteString("\n")

	// Separator
	sepWidth := m.width - 4
	if sepWidth < 10 {
		sepWidth = 40
	}

	// Calculate visible lines
	visibleLines := m.height - 8
	if visibleLines < 3 {
		visibleLines = 10
	}

	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff"))
	historyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("#7aa2f7"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#414868"))
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))

	historyLen := len(m.filteredHistory)

	// History section
	if historyLen > 0 {
		b.WriteString("  " + sectionStyle.Render("── Recently Used "+strings.Repeat("─", sepWidth-18)))
		b.WriteString("\n")
		for i, entry := range m.filteredHistory {
			if i == m.cursor {
				padded := "▸ " + entry.DisplayPath
				availWidth := m.width - 4
				if availWidth > 0 && len(padded) < availWidth {
					padded += strings.Repeat(" ", availWidth-len(padded))
				}
				b.WriteString("  " + selectedStyle.Render(padded))
			} else {
				b.WriteString("    " + historyStyle.Render(entry.DisplayPath))
			}
			b.WriteString("\n")
		}
	}

	// Directories section
	b.WriteString("  " + sectionStyle.Render("── Directories "+strings.Repeat("─", sepWidth-16)))
	b.WriteString("\n")

	if len(m.filtered) == 0 {
		b.WriteString("  " + dimStyle.Render("(empty)"))
		b.WriteString("\n")
	} else {
		historyHeight := 0
		if historyLen > 0 {
			historyHeight = historyLen + 1
		}
		dirVisibleLines := visibleLines - historyHeight - 1
		dirVisibleLines = max(dirVisibleLines, 3)

		dirOffset := 0
		if m.cursor >= historyLen {
			dirCursor := m.cursor - historyLen
			if dirCursor >= dirOffset+dirVisibleLines {
				dirOffset = dirCursor - dirVisibleLines + 1
			}
			if dirCursor < dirOffset {
				dirOffset = dirCursor
			}
		}

		end := dirOffset + dirVisibleLines
		end = min(end, len(m.filtered))
		for i := dirOffset; i < end; i++ {
			entry := m.filtered[i]
			displayName := entry + "/"
			globalIdx := historyLen + i

			if globalIdx == m.cursor {
				padded := "▸ " + displayName
				availWidth := m.width - 4
				if availWidth > 0 && len(padded) < availWidth {
					padded += strings.Repeat(" ", availWidth-len(padded))
				}
				b.WriteString("  " + selectedStyle.Render(padded))
			} else {
				b.WriteString("    " + dirStyle.Render(displayName))
			}
			b.WriteString("\n")
		}
	}

	// Footer hints
	b.WriteString("\n")
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	hiddenHint := "Ctrl+H:show hidden"
	if m.showHidden {
		hiddenHint = "Ctrl+H:hide hidden"
	}
	b.WriteString("  " + hintStyle.Render("Enter:open  Tab:select  Backspace:parent  ←/→:section  "+hiddenHint))

	return b.String()
}

// --- Internal ---

func (m *DirPickerModel) loadEntries() {
	m.entries = nil
	m.filtered = nil

	dirEntries, err := os.ReadDir(m.currentDir)
	if err != nil {
		return
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !m.showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		m.entries = append(m.entries, name)
	}

	sort.Strings(m.entries)
	m.applyFilter()
}

func (m *DirPickerModel) applyFilter() {
	query := strings.ToLower(m.filterInput.Value())
	if query == "" || strings.HasPrefix(query, "/") || strings.HasPrefix(query, "~") {
		m.filtered = m.entries
	} else {
		m.filtered = nil
		for _, e := range m.entries {
			if strings.Contains(strings.ToLower(e), query) {
				m.filtered = append(m.filtered, e)
			}
		}
	}
	if m.cursor >= m.totalItems() {
		m.cursor = 0
	}
	m.offset = 0
}

func (m *DirPickerModel) updateFilteredHistory() {
	if len(m.historyDirs) == 0 {
		m.filteredHistory = nil
		return
	}
	query := strings.ToLower(m.filterInput.Value())
	if query == "" || strings.HasPrefix(query, "/") || strings.HasPrefix(query, "~") {
		m.filteredHistory = m.historyDirs
	} else {
		m.filteredHistory = nil
		for _, h := range m.historyDirs {
			if strings.Contains(strings.ToLower(h.DisplayPath), query) {
				m.filteredHistory = append(m.filteredHistory, h)
			}
		}
	}
	if m.cursor >= m.totalItems() {
		m.cursor = 0
	}
}

// removeFilteredHistoryEntry removes a history entry at the given index from
// both filteredHistory and historyDirs, adjusting the cursor as needed.
func (m *DirPickerModel) removeFilteredHistoryEntry(idx int) {
	if idx < 0 || idx >= len(m.filteredHistory) {
		return
	}
	removed := m.filteredHistory[idx]
	m.filteredHistory = append(m.filteredHistory[:idx], m.filteredHistory[idx+1:]...)

	for i, h := range m.historyDirs {
		if h.Path == removed.Path {
			m.historyDirs = append(m.historyDirs[:i], m.historyDirs[i+1:]...)
			break
		}
	}

	if m.cursor >= m.totalItems() && m.cursor > 0 {
		m.cursor--
	}
}

func (m *DirPickerModel) adjustScroll() {
	visibleLines := m.height - 8
	if visibleLines < 3 {
		visibleLines = 10
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleLines {
		m.offset = m.cursor - visibleLines + 1
	}
}
