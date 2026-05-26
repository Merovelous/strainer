package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var archiveExts = map[string]bool{
	".7z": true, ".zip": true, ".tar": true, ".gz": true,
	".bz2": true, ".xz": true, ".tar.gz": true, ".tar.bz2": true,
	".tar.xz": true, ".tgz": true, ".rar": true,
}

func isArchiveFile(name string) bool {
	lower := strings.ToLower(name)
	// Check compound extensions first
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return archiveExts[filepath.Ext(lower)]
}

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
			isArchive: !de.IsDir() && isArchiveFile(name),
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
				if b.cursor < b.offset {
					b.offset = b.cursor
				}
			}
		case "down", "j":
			if len(b.entries) > 0 {
				b.cursor = (b.cursor + 1) % len(b.entries)
				visible := b.windowHeight - 9
				if visible < 5 {
					visible = 20
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

func (b browserModel) View(maxHeight int) string {
	if !b.ready {
		return sDim.Render("  Loading...")
	}

	// Header
	var posStr string
	if len(b.entries) > 0 {
		posStr = sDim.Render(fmt.Sprintf("  %d / %d", b.cursor+1, len(b.entries)))
	}
	header := sHeader.Render("  📁 FILE BROWSER") + posStr
	dir := sDim.Render("  " + b.currentDir)
	lines := []string{"", header, dir, ""}

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
		cursor := "  "
		if i == b.cursor {
			cursor = sPrompt.Render("▸ ")
		}

		var icon, name, sizeStr string
		if e.isDir {
			icon = sCyan.Render("📁")
			name = sCyan.Render(e.name + "/")
		} else if e.isArchive {
			icon = sMagenta.Render("📦")
			name = sMagenta.Render(e.name)
			sizeStr = sDim.Render("  " + humanSize(e.size))
		} else {
			icon = "  "
			name = e.name
			sizeStr = sDim.Render("  " + humanSize(e.size))
		}

		line := fmt.Sprintf("%s%s %s%s", cursor, icon, name, sizeStr)
		if i == b.cursor {
			line = sHighlight.Render(strings.TrimLeft(line, " "))
			line = " " + line
		}
		lines = append(lines, line)
	}

	if b.err != nil {
		lines = append(lines, sError.Render("  Error: "+b.err.Error()))
	} else if len(b.entries) == 0 {
		lines = append(lines, sDim.Render("  (empty directory)"))
	}

	// Footer
	lines = append(lines, "")
	footer := sDim.Render("  [↑↓] Navigate  [Enter] Select  [Backspace] Parent dir  [q] Quit")
	lines = append(lines, footer)

	return strings.Join(lines, "\n")
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
