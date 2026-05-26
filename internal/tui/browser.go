package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Merovelous/strainer/internal/pipeline"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newBrowserModel(dir string) browserModel {
	bm := browserModel{currentDir: dir}
	return bm
}

func (b browserModel) Init() tea.Cmd {
	dir := b.currentDir
	return func() tea.Msg {
		entries, err := listEntries(dir)
		return browserReadyMsg{entries: entries, err: err}
	}
}

func listEntries(dir string) ([]entry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var entries []entry
	for _, de := range dirEntries {
		name := de.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := de.Info()
		var size int64
		if err == nil {
			size = info.Size()
		}
		e := entry{
			name:      name,
			isDir:     de.IsDir(),
			isArchive: !de.IsDir() && pipeline.IsArchiveFile(name),
			size:      size,
			path:      filepath.Join(dir, name),
		}
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	return entries, nil
}

func (b browserModel) Update(msg tea.Msg) (browserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case browserReadyMsg:
		b.err = msg.err
		b.entries = msg.entries
		b.cursor = 0
		b.offset = 0
		b.ready = true
		return b, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if len(b.entries) > 0 {
				b.cursor = (b.cursor - 1 + len(b.entries)) % len(b.entries)
				if b.cursor == len(b.entries)-1 {
					// Wrapped to bottom — scroll viewport to show last entry
					visible := b.windowHeight - 11
					if visible < 5 {
						visible = 5
					}
					b.offset = len(b.entries) - visible
					if b.offset < 0 {
						b.offset = 0
					}
				} else if b.cursor < b.offset {
					b.offset = b.cursor
				}
			}
		case "down", "j":
			if len(b.entries) > 0 {
				b.cursor = (b.cursor + 1) % len(b.entries)
				visible := b.windowHeight - 11
				if visible < 5 {
					visible = 5
				}
				if b.cursor == 0 {
					b.offset = 0
				} else if b.cursor >= b.offset+visible {
					b.offset = b.cursor - visible + 1
				}
			}
		case "enter":
			if len(b.entries) == 0 {
				return b, nil
			}
			e := b.entries[b.cursor]
			if e.isDir {
				b.currentDir = e.path
				b.cursor = 0
				b.offset = 0
				b.entries = nil
				b.err = nil
				dir := e.path
				return b, func() tea.Msg {
					entries, err := listEntries(dir)
					return browserReadyMsg{entries: entries, err: err}
				}
			}
			// File selected
			return b, func() tea.Msg {
				return browserSelectMsg{path: e.path, isArchive: e.isArchive}
			}
		case "backspace":
			parent := filepath.Dir(b.currentDir)
			if parent != b.currentDir {
				b.currentDir = parent
				b.cursor = 0
				b.offset = 0
				b.entries = nil
				b.err = nil
				return b, func() tea.Msg {
					entries, err := listEntries(parent)
					return browserReadyMsg{entries: entries, err: err}
				}
			}
		}
	}
	return b, nil
}

func (b browserModel) View(width, maxHeight int) string {
	if !b.ready {
		return "\n" + sDim.Render("  Loading directory...")
	}

	listWidth := width - 2
	if listWidth < 20 {
		listWidth = 20
	}

	// Path header
	dirLabel := sDim.Render("  " + b.currentDir)
	var posStr string
	if len(b.entries) > 0 {
		posStr = sDimmer.Render(fmt.Sprintf("  %d/%d", b.cursor+1, len(b.entries)))
	}
	lines := []string{"", dirLabel + posStr, ""}

	// Entries
	visible := maxHeight - 6
	if visible < 5 {
		visible = 5
	}
	start := b.offset
	end := start + visible
	if end > len(b.entries) {
		end = len(b.entries)
	}

	for i := start; i < end; i++ {
		e := b.entries[i]
		selected := i == b.cursor

		var icon, nameStr, sizeStr string
		switch {
		case e.isDir:
			icon = sCyan.Render(" ▶ ")
			nameStr = sCyan.Render(e.name + "/")
		case e.isArchive:
			icon = sMagenta.Render(" ◈ ")
			nameStr = sMagenta.Render(e.name)
			sizeStr = pipeline.HumanSize(e.size)
		default:
			icon = "   "
			nameStr = e.name
			sizeStr = pipeline.HumanSize(e.size)
		}

		innerLeft := "  " + icon + " " + nameStr
		innerGap := listWidth - lipgloss.Width(innerLeft) - len(sizeStr) - 2
		if innerGap < 1 {
			innerGap = 1
		}

		var row string
		if selected {
			rowStyle := sRowSelected.Width(listWidth)
			row = rowStyle.Render(innerLeft + strings.Repeat(" ", innerGap) + sizeStr)
		} else {
			row = innerLeft + strings.Repeat(" ", innerGap) + sDim.Render(sizeStr)
		}

		lines = append(lines, row)
	}

	if b.err != nil {
		lines = append(lines, "")
		lines = append(lines, sError.Render("  ✖ "+b.err.Error()))
	} else if len(b.entries) == 0 {
		lines = append(lines, sDim.Render("  (empty directory)"))
	}

	// Scroll indicator
	if len(b.entries) > visible {
		above := b.offset > 0
		below := b.offset+visible < len(b.entries)
		var scroll string
		if above && below {
			scroll = "↑↓"
		} else if above {
			scroll = "↑ "
		} else {
			scroll = " ↓"
		}
		lines = append(lines, sDimmer.Render(fmt.Sprintf("  %s  %d more", scroll, len(b.entries)-visible)))
	}

	return strings.Join(lines, "\n")
}
