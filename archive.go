package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// sevenZipBin is detected once at startup — "7zz" on macOS, "7z" on Linux.
var sevenZipBin = func() string {
	for _, name := range []string{"7z", "7zz", "7za"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "7z" // fall through to a clear error message at runtime
}()

func newArchivePickerModel(archivePath string) (archivePickerModel, tea.Cmd) {
	ap := archivePickerModel{archivePath: archivePath, loading: true}
	return ap, ap.loadEntries()
}

func (ap archivePickerModel) loadEntries() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, sevenZipBin, "l", ap.archivePath)
		out, err := cmd.Output()
		if err != nil {
			if ctx.Err() != nil {
				return archiveReadyMsg{err: fmt.Errorf("archive scan timed out (30s)")}
			}
			return archiveReadyMsg{err: fmt.Errorf("%s l failed: %w\n%s", sevenZipBin, err, string(out))}
		}
		entries := parse7zListing(string(out))
		return archiveReadyMsg{entries: entries}
	}
}

func parse7zListing(output string) []archiveEntry {
	lines := strings.Split(output, "\n")

	// Find the first separator line (only dashes and spaces, ≥10 chars).
	// The separator line itself tells us where the name column starts — the
	// last run of '-' chars begins at the name column offset.
	sepIdx := -1
	nameColStart := -1
	for i, line := range lines {
		if isSepLine(line) {
			sepIdx = i
			nameColStart = findNameCol(line)
			break
		}
	}
	if sepIdx == -1 {
		return nil
	}

	var entries []archiveEntry
	for i := sepIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isSepLine(line) || isSummaryLine(trimmed) {
			break
		}

		var name string
		if nameColStart > 0 && len(line) > nameColStart {
			name = strings.TrimSpace(line[nameColStart:])
		} else {
			// Fallback: last whitespace field (breaks on filenames with spaces)
			fields := strings.Fields(trimmed)
			if len(fields) > 0 {
				name = fields[len(fields)-1]
			}
		}
		if name == "" {
			continue
		}
		entries = append(entries, archiveEntry{name: name, index: len(entries)})
	}
	return entries
}

// isSepLine detects a 7z column separator: only dashes and spaces, ≥10 chars.
func isSepLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) >= 10 && strings.Trim(trimmed, "- ") == ""
}

// findNameCol returns the index where the last run of '-' chars begins in the
// separator line — that position is the start of the filename column.
func findNameCol(sepLine string) int {
	inDash, lastStart := false, -1
	for i, ch := range sepLine {
		if ch == '-' {
			if !inDash {
				inDash = true
				lastStart = i
			}
		} else {
			inDash = false
		}
	}
	return lastStart
}

// isSummaryLine detects the post-listing summary row, e.g. "1 files, 0 folders".
// It must start with a digit to avoid matching filenames that contain "files".
func isSummaryLine(trimmed string) bool {
	if len(trimmed) == 0 || trimmed[0] < '0' || trimmed[0] > '9' {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "file") || strings.Contains(lower, "folder")
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
