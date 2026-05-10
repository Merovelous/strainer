package main

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func newFilterModel(fileName string) filterModel {
	return filterModel{
		fileName: fileName,
		options: []filterOption{
			{name: "Min Length", enabled: false, value: 0, dynamic: true},
			{name: "Max Length", enabled: false, value: 0, dynamic: true},
			{name: "ASCII Only", enabled: false, value: 0, dynamic: false},
		},
		ready: true,
	}
}

func (f filterModel) getMinLen() int {
	if f.options[0].enabled {
		return f.options[0].value
	}
	return 0
}

func (f filterModel) getMaxLen() int {
	if f.options[1].enabled {
		return f.options[1].value
	}
	return 0
}

func (f filterModel) isASCIIOnly() bool {
	return f.options[2].enabled
}

func (f filterModel) buildOutputName(inputPath string) string {
	// Get just the filename from path
	parts := strings.Split(inputPath, "/")
	base := parts[len(parts)-1]

	// Strip any extension (archive or regular)
	// Check compound archive extensions first
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(strings.ToLower(base), ext) {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	// Strip single extension
	if idx := strings.LastIndex(base, "."); idx > 0 {
		base = base[:idx]
	}

	var suffix string
	minLen := f.getMinLen()
	maxLen := f.getMaxLen()
	if minLen > 0 && maxLen > 0 {
		suffix += fmt.Sprintf("_%dto%d", minLen, maxLen)
	} else if minLen > 0 {
		suffix += fmt.Sprintf("_min%d", minLen)
	} else if maxLen > 0 {
		suffix += fmt.Sprintf("_max%d", maxLen)
	}
	if f.isASCIIOnly() {
		suffix += "_ascii"
	}
	if suffix == "" {
		suffix = "_filtered"
	}

	return base + suffix + ".txt"
}

func (f filterModel) Init() tea.Cmd {
	return func() tea.Msg { return filterReadyMsg{} }
}

func (f filterModel) Update(msg tea.Msg) (filterModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if f.inputing {
			return f.handleInput(msg)
		}
		switch msg.String() {
		case "up", "k":
			if f.cursor > 0 {
				f.cursor--
			}
		case "down", "j":
			if f.cursor < len(f.options)-1 {
				f.cursor++
			}
		case "enter", " ":
			opt := &f.options[f.cursor]
			if opt.dynamic && !opt.enabled {
				// Enable and start input
				opt.enabled = true
				f.inputing = true
				f.inputIdx = f.cursor
				f.inputBuf = ""
			} else if opt.dynamic && opt.enabled {
				// Toggle off
				opt.enabled = false
				opt.value = 0
			} else {
				// Toggle non-dynamic
				opt.enabled = !opt.enabled
			}
		case "e":
			// Edit value of current option
			opt := &f.options[f.cursor]
			if opt.dynamic && opt.enabled {
				f.inputing = true
				f.inputIdx = f.cursor
				f.inputBuf = strconv.Itoa(opt.value)
			}
		case "tab":
			// Confirm and move to processing
			return f, func() tea.Msg {
				return filterConfirmMsg{}
			}
		}
	}
	return f, nil
}

type filterConfirmMsg struct{}

func (f filterModel) handleInput(msg tea.KeyMsg) (filterModel, tea.Cmd) {
	opt := &f.options[f.inputIdx]
	switch msg.String() {
	case "enter":
		val, err := strconv.Atoi(f.inputBuf)
		if err == nil && val > 0 {
			opt.value = val
		} else {
			opt.enabled = false
			opt.value = 0
		}
		f.inputing = false
		f.inputBuf = ""
	case "esc":
		f.inputing = false
		f.inputBuf = ""
		if opt.value == 0 {
			opt.enabled = false
		}
	case "backspace":
		if len(f.inputBuf) > 0 {
			f.inputBuf = f.inputBuf[:len(f.inputBuf)-1]
		}
	default:
		if len(msg.String()) == 1 && msg.String() >= "0" && msg.String() <= "9" {
			f.inputBuf += msg.String()
		}
	}
	return f, nil
}

func (f filterModel) View(width, maxHeight int) string {
	header := sHeader.Render("  ⚙️  FILTER CONFIGURATION")
	file := sDim.Render("  File: " + f.fileName)
	lines := []string{"", header, file, ""}

	for i, opt := range f.options {
		cursor := "  "
		if i == f.cursor {
			cursor = sPrompt.Render("▸ ")
		}

		// Checkbox
		check := sError.Render("☐")
		if opt.enabled {
			check = sSuccess.Render("☑")
		}

		// Name
		name := opt.name
		if i == f.cursor && !f.inputing {
			name = sSelected.Render(name)
		} else if opt.enabled {
			name = sSuccess.Render(name)
		} else {
			name = sDim.Render(name)
		}

		// Value
		valStr := ""
		if opt.dynamic {
			if f.inputing && f.inputIdx == i {
				valStr = sValue.Render(" [" + f.inputBuf + "█]")
			} else if opt.enabled && opt.value > 0 {
				valStr = sValue.Render(fmt.Sprintf(" [%d]", opt.value))
			} else {
				valStr = sDim.Render(" [press Enter to set]")
			}
		}

		lines = append(lines, fmt.Sprintf("%s%s %s%s", cursor, check, name, valStr))
	}

	// Output filename preview
	lines = append(lines, "")
	outputName := f.buildOutputName(f.fileName)
	lines = append(lines, sSubHeader.Render("  Output: ")+sValue.Render(outputName))

	// Rules summary
	lines = append(lines, "")
	var rules []string
	if f.getMinLen() > 0 {
		rules = append(rules, fmt.Sprintf("min=%d", f.getMinLen()))
	}
	if f.getMaxLen() > 0 {
		rules = append(rules, fmt.Sprintf("max=%d", f.getMaxLen()))
	}
	if f.isASCIIOnly() {
		rules = append(rules, "ascii-only")
	}
	if len(rules) > 0 {
		lines = append(lines, sDim.Render("  Rules: ")+sSuccess.Render(strings.Join(rules, ", ")))
	} else {
		lines = append(lines, sWarning.Render("  No filters configured"))
	}

	// Footer
	lines = append(lines, "")
	footer := sDim.Render("  [↑↓] Navigate  [Enter/Space] Toggle  [e] Edit value  [Tab] Start processing  [q] Quit")
	lines = append(lines, footer)

	return strings.Join(lines, "\n")
}
