package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// bloomPresets maps choiceIdx → filter size in bytes; index 0 = disabled.
var bloomPresets = []int64{0, 256 << 20, 512 << 20, 1 << 30, 4 << 30, 8 << 30}

func newFilterModel(fileName string, fileSize int64) filterModel {
	return filterModel{
		fileName: fileName,
		fileSize: fileSize,
		options: []filterOption{
			{name: "Min Length", dynamic: true},
			{name: "Max Length", dynamic: true},
			{name: "ASCII Only"},
			{name: "Regex Match", strDynamic: true},
			{name: "Deduplicate"},
			{name: "Bloom Filter Size", cycle: true, choices: []string{"Off", "256 MB", "512 MB", "1 GB", "4 GB", "8 GB"}},
		},
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

func (f filterModel) getRegex() *regexp.Regexp {
	if !f.options[3].enabled || f.options[3].strValue == "" {
		return nil
	}
	re, err := regexp.Compile(f.options[3].strValue)
	if err != nil {
		return nil
	}
	return re
}

func (f filterModel) getRegexStr() string {
	if f.options[3].enabled {
		return f.options[3].strValue
	}
	return ""
}

func (f filterModel) isDeduplicate() bool {
	return f.options[4].enabled
}

func (f filterModel) getBloomSize() int64 {
	opt := f.options[5]
	if opt.choiceIdx <= 0 || opt.choiceIdx >= len(bloomPresets) {
		return 0
	}
	return bloomPresets[opt.choiceIdx]
}

func (f filterModel) buildOutputName(inputPath string) string {
	base := filepath.Base(inputPath)

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
	if f.getRegexStr() != "" {
		suffix += "_regex"
	}
	if f.isDeduplicate() {
		suffix += "_dedup"
	}
	if suffix == "" {
		suffix = "_filtered"
	}

	return base + suffix + ".txt"
}

func (f filterModel) Update(msg tea.Msg) (filterModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if f.inputing {
			return f.handleInput(msg)
		}
		total := len(f.options) + 1 // +1 for the Start button at the bottom
		switch msg.String() {
		case "up", "k":
			f.cursor = (f.cursor - 1 + total) % total
		case "down", "j":
			f.cursor = (f.cursor + 1) % total
		case "left":
			if f.cursor < len(f.options) {
				opt := &f.options[f.cursor]
				if opt.cycle && len(opt.choices) > 0 {
					opt.choiceIdx = (opt.choiceIdx - 1 + len(opt.choices)) % len(opt.choices)
				}
			}
		case "right":
			if f.cursor < len(f.options) {
				opt := &f.options[f.cursor]
				if opt.cycle && len(opt.choices) > 0 {
					opt.choiceIdx = (opt.choiceIdx + 1) % len(opt.choices)
				}
			}
		case "enter", " ":
			if f.cursor == len(f.options) {
				return f.confirm()
			}
			opt := &f.options[f.cursor]
			switch {
			case opt.cycle:
				if len(opt.choices) > 0 {
					opt.choiceIdx = (opt.choiceIdx + 1) % len(opt.choices)
				}
			case opt.strDynamic && !opt.enabled:
				opt.enabled = true
				f.inputing = true
				f.inputIdx = f.cursor
				f.inputBuf = ""
			case opt.strDynamic && opt.enabled:
				opt.enabled = false
				opt.strValue = ""
			case opt.dynamic && !opt.enabled:
				opt.enabled = true
				f.inputing = true
				f.inputIdx = f.cursor
				f.inputBuf = ""
			case opt.dynamic && opt.enabled:
				opt.enabled = false
				opt.value = 0
			default:
				opt.enabled = !opt.enabled
			}
		case "e":
			if f.cursor == len(f.options) {
				return f, nil
			}
			opt := &f.options[f.cursor]
			if opt.strDynamic && opt.enabled {
				f.inputing = true
				f.inputIdx = f.cursor
				f.inputBuf = opt.strValue
			} else if opt.dynamic && opt.enabled {
				f.inputing = true
				f.inputIdx = f.cursor
				f.inputBuf = strconv.Itoa(opt.value)
			}
		case "tab":
			return f.confirm()
		}
	}
	return f, nil
}

type filterConfirmMsg struct{}

func (f filterModel) confirm() (filterModel, tea.Cmd) {
	minLen := f.getMinLen()
	maxLen := f.getMaxLen()
	if minLen > 0 && maxLen > 0 && minLen > maxLen {
		f.validationErr = fmt.Sprintf("min length (%d) must be ≤ max length (%d)", minLen, maxLen)
		return f, nil
	}
	f.validationErr = ""
	return f, func() tea.Msg { return filterConfirmMsg{} }
}

func (f filterModel) handleInput(msg tea.KeyMsg) (filterModel, tea.Cmd) {
	opt := &f.options[f.inputIdx]
	isStr := opt.strDynamic
	switch msg.String() {
	case "enter":
		if isStr {
			if f.inputBuf != "" {
				if _, err := regexp.Compile(f.inputBuf); err != nil {
					f.validationErr = "invalid regex: " + err.Error()
					return f, nil
				}
				opt.strValue = f.inputBuf
				f.validationErr = ""
			} else {
				opt.enabled = false
				opt.strValue = ""
			}
		} else {
			val, err := strconv.Atoi(f.inputBuf)
			if err == nil && val > 0 {
				opt.value = val
			} else {
				opt.enabled = false
				opt.value = 0
			}
		}
		f.inputing = false
		f.inputBuf = ""
	case "esc":
		f.inputing = false
		f.inputBuf = ""
		if isStr {
			if opt.strValue == "" {
				opt.enabled = false
			}
		} else {
			if opt.value == 0 {
				opt.enabled = false
			}
		}
	case "backspace":
		if len(f.inputBuf) > 0 {
			f.inputBuf = f.inputBuf[:len(f.inputBuf)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 {
			if isStr {
				if ch[0] >= 32 && ch[0] < 127 {
					f.inputBuf += ch
				}
			} else if ch >= "0" && ch <= "9" {
				f.inputBuf += ch
			}
		}
	}
	return f, nil
}

// bloomAnnotation returns the FPR estimate and RAM availability hint for a given bloom size.
func (f filterModel) bloomAnnotation(sizeBytes int64) string {
	var fprStr string
	if f.fileSize > 0 {
		estLines := f.fileSize / 10
		if estLines < 1 {
			estLines = 1
		}
		fpr := bloomFPR(sizeBytes, estLines)
		if fpr < 0.0001 {
			fprStr = sDim.Render("  FPR < 0.01%")
		} else {
			fprStr = sDim.Render(fmt.Sprintf("  FPR ~%.2f%%", fpr*100))
		}
	}
	avail, ok := availableRAM()
	var ramStr string
	if ok {
		if sizeBytes > avail*8/10 {
			ramStr = sError.Render(fmt.Sprintf("  ⚠ need %s, %s free", humanSize(sizeBytes), humanSize(avail)))
		} else {
			ramStr = sDim.Render(fmt.Sprintf("  (%s free)", humanSize(avail)))
		}
	}
	return fprStr + ramStr
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

		// Derive enabled state (cycle options use choiceIdx)
		isEnabled := opt.enabled || (opt.cycle && opt.choiceIdx > 0)

		// Checkbox
		check := sError.Render("☐")
		if isEnabled {
			check = sSuccess.Render("☑")
		}

		// Name
		name := opt.name
		if i == f.cursor && !f.inputing {
			name = sSelected.Render(name)
		} else if isEnabled {
			name = sSuccess.Render(name)
		} else {
			name = sDim.Render(name)
		}

		// Value
		valStr := ""
		if opt.strDynamic {
			if f.inputing && f.inputIdx == i {
				valStr = sValue.Render(" [" + f.inputBuf + "█]")
			} else if opt.enabled && opt.strValue != "" {
				valStr = sValue.Render(" [" + opt.strValue + "]")
			} else {
				valStr = sDim.Render(" [press Enter to set]")
			}
		} else if opt.dynamic {
			if f.inputing && f.inputIdx == i {
				valStr = sValue.Render(" [" + f.inputBuf + "█]")
			} else if opt.enabled && opt.value > 0 {
				valStr = sValue.Render(fmt.Sprintf(" [%d]", opt.value))
			} else {
				valStr = sDim.Render(" [press Enter to set]")
			}
		} else if opt.cycle {
			choice := ""
			if opt.choiceIdx < len(opt.choices) {
				choice = opt.choices[opt.choiceIdx]
			}
			arrowStr := fmt.Sprintf(" [← %s →]", choice)
			if !f.isDeduplicate() {
				valStr = sDim.Render(arrowStr + "  (enable Deduplicate first)")
			} else if opt.choiceIdx > 0 && opt.choiceIdx < len(bloomPresets) {
				valStr = sValue.Render(arrowStr) + f.bloomAnnotation(bloomPresets[opt.choiceIdx])
			} else {
				valStr = sDim.Render(arrowStr)
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
	if f.getRegexStr() != "" {
		rules = append(rules, "regex")
	}
	if f.isDeduplicate() {
		rules = append(rules, "dedup")
	}
	if len(rules) > 0 {
		lines = append(lines, sDim.Render("  Rules: ")+sSuccess.Render(strings.Join(rules, ", ")))
	} else {
		lines = append(lines, sWarning.Render("  No filters configured"))
	}

	// Start button
	lines = append(lines, "")
	if f.cursor == len(f.options) && !f.inputing {
		lines = append(lines, sPrompt.Render("▸ ")+sHighlight.Render(" ▶  Start Processing "))
	} else {
		lines = append(lines, sDim.Render("    ▶  Start Processing"))
	}

	// Validation error
	if f.validationErr != "" {
		lines = append(lines, "")
		lines = append(lines, sError.Render("  ✖ "+f.validationErr))
	}

	// Footer
	lines = append(lines, "")
	footer := sDim.Render("  [↑↓] Navigate  [Enter/Space] Toggle  [←→] Cycle  [e] Edit value  [Tab] Start  [q] Quit")
	lines = append(lines, footer)

	return strings.Join(lines, "\n")
}
