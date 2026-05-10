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
			return archiveReadyMsg{}
		}
		ap.entries = parse7zListing(out)
		return archiveReadyMsg{}
	}
}

func parse7zListing(output string) []archiveEntry {
	lines := strings.Split(output, "\n")

	// Find the dash line separator (e.g., "------------------- -----")
	dashIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= 15 && strings.Trim(trimmed, "-") == "" {
			dashIdx = i
			break
		}
	}
	if dashIdx == -1 {
		return nil
	}

	// Everything after the dash line is file listing
	rest := strings.Join(lines[dashIdx+1:], "\n")
	// Split by blank line to get entries
	blocks := strings.Split(rest, "\n\n")
	var entries []archiveEntry
	for idx, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		// Extract filename (last column after whitespace)
		parts := strings.Fields(block)
		if len(parts) < 6 {
			continue
		}
		name := parts[len(parts)-1]
		if name == "" {
			continue
		}
		entries = append(entries, archiveEntry{name: name, index: idx})
	}
	return entries
}

func (ap archivePickerModel) Update(msg tea.Msg) (archivePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case archiveReadyMsg:
		ap.loading = false
		ap.done = true
		return ap, nil

	case tea.KeyMsg:
		if ap.loading {
			return ap, nil
		}
		switch msg.String() {
		case "up", "k":
			if ap.cursor > 0 {
				ap.cursor--
				if ap.cursor < ap.offset {
					ap.offset = ap.cursor
				}
			}
		case "down", "j":
			if ap.cursor < len(ap.entries)-1 {
				ap.cursor++
				visible := 20
				if ap.cursor >= ap.offset+visible {
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
	// Header
	header := sHeader.Render("  📦 ARCHIVE CONTENTS")
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
		lines = append(lines, sDim.Render("  (no files found in archive)"))
		return strings.Join(lines, "\n")
	}

	// Entries
	visible := maxHeight - 8
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
