package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func newArchivePickerModel(archivePath string) (archivePickerModel, tea.Cmd) {
	ap := archivePickerModel{archivePath: archivePath, loading: true}
	return ap, ap.loadEntries()
}

func (ap archivePickerModel) loadEntries() tea.Cmd {
	return func() tea.Msg {
		out, err := runCommand("7z", "l", ap.archivePath)
		if err != nil {
			return archiveReadyMsg{err: fmt.Errorf("7z l failed: %w\n%s", err, out)}
		}
		entries := parse7zListing(out)
		return archiveReadyMsg{entries: entries}
	}
}

func parse7zListing(output string) []archiveEntry {
	lines := strings.Split(output, "\n")

	// Find the separator line: "---... ---..." (dashes with spaces)
	sepIdx := -1
	nameColStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= 15 && strings.Contains(trimmed, " ") && strings.Trim(trimmed, "- ") == "" {
			sepIdx = i
			break
		}
		// Find "Name" column position in header
		if strings.Contains(trimmed, "Name") && strings.Contains(trimmed, "Size") {
			nameColStart = strings.Index(line, "Name")
		}
	}
	if sepIdx == -1 {
		return nil
	}

	// Default: filename is last whitespace-delimited field
	// But if we found the Name column position, use that for cleaner parsing
	usePosition := nameColStart > 0

	var entries []archiveEntry
	for i := sepIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		// Second dash line = end of file list
		if len(trimmed) >= 15 && strings.Contains(trimmed, " ") && strings.Trim(trimmed, "- ") == "" {
			break
		}

		// Summary line (contains "files," or "folders") = end
		if strings.Contains(trimmed, "files,") || strings.Contains(trimmed, "folders") {
			break
		}

		// Extract filename
		var name string
		if usePosition && len(line) > nameColStart {
			name = strings.TrimSpace(line[nameColStart:])
		} else {
			fields := strings.Fields(trimmed)
			if len(fields) < 5 {
				continue
			}
			name = fields[len(fields)-1]
		}

		if name == "" {
			continue
		}
		entries = append(entries, archiveEntry{name: name, index: len(entries)})
	}
	return entries
}

func (ap archivePickerModel) Update(msg tea.Msg) (archivePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case archiveReadyMsg:
		ap.loading = false
		ap.entries = msg.entries
		ap.err = msg.err
		return ap, nil

	case tea.KeyMsg:
		if ap.loading {
			return ap, nil
		}
		switch msg.String() {
		case "up", "k":
			if len(ap.entries) > 0 {
				ap.cursor = (ap.cursor - 1 + len(ap.entries)) % len(ap.entries)
				if ap.cursor == len(ap.entries)-1 {
					visible := ap.windowHeight - 11
					if visible < 5 {
						visible = 5
					}
					ap.offset = len(ap.entries) - visible
					if ap.offset < 0 {
						ap.offset = 0
					}
				} else if ap.cursor < ap.offset {
					ap.offset = ap.cursor
				}
			}
		case "down", "j":
			if len(ap.entries) > 0 {
				ap.cursor = (ap.cursor + 1) % len(ap.entries)
				visible := ap.windowHeight - 11
				if visible < 5 {
					visible = 5
				}
				if ap.cursor == 0 {
					ap.offset = 0
				} else if ap.cursor >= ap.offset+visible {
					ap.offset = ap.cursor - visible + 1
				}
			}
		case "enter":
			if len(ap.entries) == 0 {
				return ap, nil
			}
			e := ap.entries[ap.cursor]
			return ap, func() tea.Msg {
				return archiveSelectMsg{file: e.name}
			}
		}
	}
	return ap, nil
}

func (ap archivePickerModel) View(maxHeight int) string {
	var posStr string
	if len(ap.entries) > 0 {
		posStr = sDim.Render(fmt.Sprintf("  %d / %d", ap.cursor+1, len(ap.entries)))
	}
	header := sHeader.Render("  📦 ARCHIVE CONTENTS") + posStr
	archive := sDim.Render("  " + ap.archivePath)
	lines := []string{"", header, archive, ""}

	if ap.loading {
		lines = append(lines, sDim.Render("  Scanning archive..."))
		return strings.Join(lines, "\n")
	}

	if ap.err != nil {
		lines = append(lines, sError.Render("  Error: "+ap.err.Error()))
		return strings.Join(lines, "\n")
	}

	if len(ap.entries) == 0 {
		lines = append(lines, sWarning.Render("  (no files found in archive)"))
		lines = append(lines, sDim.Render("  Archive may be empty or format unsupported"))
		return strings.Join(lines, "\n")
	}

	// Entries
	visible := maxHeight - 6
	if visible < 5 {
		visible = 5
	}
	start := ap.offset
	end := start + visible
	if end > len(ap.entries) {
		end = len(ap.entries)
	}

	for i := start; i < end; i++ {
		e := ap.entries[i]
		cursor := "  "
		if i == ap.cursor {
			cursor = sPrompt.Render("▸ ")
		}

		icon := "📄"
		name := e.name
		if i == ap.cursor {
			name = sSelected.Render(name)
		}

		line := fmt.Sprintf("%s%s %s", cursor, icon, name)
		if i == ap.cursor {
			line = sHighlight.Render(strings.TrimLeft(line, " "))
			line = " " + line
		}
		lines = append(lines, line)
	}

	// Footer
	lines = append(lines, "")
	footer := sDim.Render("  [↑↓] Navigate  [Enter] Select file to process  [q] Cancel")
	lines = append(lines, footer)

	return strings.Join(lines, "\n")
}
