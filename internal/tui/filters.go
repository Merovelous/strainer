package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Merovelous/strainer/internal/pipeline"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
			{name: "Bloom Filter Size", cycle: true, choices: []string{"Off", "256 MB", "512 MB", "1 GB", "4 GB", "8 GB", "Custom"}},
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
	if opt.choiceIdx == 0 {
		return 0
	}
	customIdx := len(opt.choices) - 1
	if opt.choiceIdx == customIdx {
		return pipeline.ParseBloomSize(opt.strValue)
	}
	if opt.choiceIdx < len(bloomPresets) {
		return bloomPresets[opt.choiceIdx]
	}
	return 0
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
				customIdx := len(opt.choices) - 1
				if msg.String() == "enter" && opt.choiceIdx == customIdx {
					// Enter on Custom → open text input
					f.inputing = true
					f.inputIdx = f.cursor
					f.inputBuf = opt.strValue
				} else {
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
			if opt.cycle && opt.choiceIdx == len(opt.choices)-1 {
				f.inputing = true
				f.inputIdx = f.cursor
				f.inputBuf = opt.strValue
			} else if opt.strDynamic && opt.enabled {
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

func (f filterModel) confirm() (filterModel, tea.Cmd) {
	minLen := f.getMinLen()
	maxLen := f.getMaxLen()
	if minLen > 0 && maxLen > 0 && minLen > maxLen {
		f.validationErr = fmt.Sprintf("min length (%d) must be ≤ max length (%d)", minLen, maxLen)
		return f, nil
	}
	noFilters := minLen == 0 && maxLen == 0 && !f.isASCIIOnly() && f.getRegexStr() == "" && !f.isDeduplicate()
	if noFilters {
		f.validationErr = "no filters selected — enable at least one filter before processing"
		return f, nil
	}
	if bloomSize := f.getBloomSize(); bloomSize > 0 {
		if avail, ok := pipeline.AvailableRAM(); ok && bloomSize > avail {
			f.validationErr = fmt.Sprintf("bloom filter needs %s but only %s RAM free — choose a smaller size",
				pipeline.HumanSize(bloomSize), pipeline.HumanSize(avail))
			return f, nil
		}
	}
	f.validationErr = ""
	return f, func() tea.Msg { return filterConfirmMsg{} }
}

func (f filterModel) handleInput(msg tea.KeyMsg) (filterModel, tea.Cmd) {
	opt := &f.options[f.inputIdx]
	isStr := opt.strDynamic
	isCycle := opt.cycle
	switch msg.String() {
	case "enter":
		if isCycle {
			if f.inputBuf == "" {
				opt.strValue = ""
				opt.choiceIdx = 0 // revert to Off on empty input
			} else if pipeline.ParseBloomSize(f.inputBuf) > 0 {
				opt.strValue = f.inputBuf
				f.validationErr = ""
			} else {
				f.validationErr = "invalid size — use a number followed by m or g  (e.g. 16g, 2048m)"
				return f, nil
			}
			f.inputing = false
			f.inputBuf = ""
			return f, nil
		}
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
		f.validationErr = ""
		if isCycle {
			if opt.strValue == "" {
				opt.choiceIdx = 0 // revert to Off if nothing was saved
			}
		} else if isStr {
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
			if isCycle {
				// Accept digits and unit suffixes only
				if (ch >= "0" && ch <= "9") || ch == "g" || ch == "G" || ch == "m" || ch == "M" {
					f.inputBuf += ch
				}
			} else if isStr {
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
		fpr := pipeline.BloomFPR(sizeBytes, estLines)
		if fpr < 0.0001 {
			fprStr = sDim.Render("  FPR < 0.01%")
		} else {
			fprStr = sDim.Render(fmt.Sprintf("  FPR ~%.2f%%", fpr*100))
		}
	}
	avail, ok := pipeline.AvailableRAM()
	var ramStr string
	if ok {
		if sizeBytes > avail {
			ramStr = sError.Render(fmt.Sprintf("  ✖ need %s, only %s free", pipeline.HumanSize(sizeBytes), pipeline.HumanSize(avail)))
		} else if sizeBytes > avail*8/10 {
			ramStr = sWarning.Render(fmt.Sprintf("  ⚠ need %s, %s free", pipeline.HumanSize(sizeBytes), pipeline.HumanSize(avail)))
		} else {
			ramStr = sDim.Render(fmt.Sprintf("  (%s free)", pipeline.HumanSize(avail)))
		}
	}
	return fprStr + ramStr
}

// renderSectionDivider renders "  ─  NAME  ──────" filling to width.
func renderSectionDivider(name string, width int) string {
	label := " " + name + " "
	// visible chars: 2(margin) + 1(─) + len(label) = 3+len(label)
	dashCount := width - 3 - len(label) - 2
	if dashCount < 2 {
		dashCount = 2
	}
	return sDimmer.Render("  ─") + sSection.Render(label) + sDimmer.Render(strings.Repeat("─", dashCount))
}

// renderOptValue returns the value indicator string for a filter option.
func (f filterModel) renderOptValue(i int) string {
	opt := f.options[i]
	isTyping := f.inputing && f.inputIdx == i

	switch {
	case opt.dynamic:
		if isTyping {
			return sValue.Render(fmt.Sprintf("[ %-5s]", f.inputBuf+"█"))
		}
		if opt.enabled && opt.value > 0 {
			return sValue.Render(fmt.Sprintf("[ %-5d]", opt.value))
		}
		return sDim.Render("[  ─   ]")

	case opt.strDynamic:
		if isTyping {
			disp := f.inputBuf
			if len(disp) > 10 {
				disp = "…" + disp[len(disp)-9:]
			}
			return sValue.Render("[ " + disp + "█ ]")
		}
		if opt.enabled && opt.strValue != "" {
			disp := opt.strValue
			if len(disp) > 10 {
				disp = "…" + disp[len(disp)-9:]
			}
			return sValue.Render("[ " + disp + " ]")
		}
		return sDim.Render("[  ─   ]")

	case opt.cycle:
		// Bloom size — nested under Deduplicate.
		if !f.isDeduplicate() {
			return sDimmer.Render("[← off →]") + sDimmer.Render("  enable Deduplicate first")
		}
		customIdx := len(opt.choices) - 1
		isCustom := opt.choiceIdx == customIdx
		choice := ""
		if opt.choiceIdx < len(opt.choices) {
			choice = opt.choices[opt.choiceIdx]
		}
		if isCustom {
			if isTyping {
				return sValue.Render(fmt.Sprintf("[← %s█ →]", f.inputBuf))
			}
			if opt.strValue != "" {
				parsed := pipeline.ParseBloomSize(opt.strValue)
				if parsed > 0 {
					return sValue.Render(fmt.Sprintf("[← %s →]", opt.strValue)) + f.bloomAnnotation(parsed)
				}
				return sError.Render(fmt.Sprintf("[← %s →]", opt.strValue)) + sError.Render("  invalid — use e.g. 16g, 2048m")
			}
			return sValue.Render("[← Custom →]") + sDim.Render("  press Enter to type size")
		}
		if opt.choiceIdx > 0 && opt.choiceIdx < len(bloomPresets) {
			return sValue.Render(fmt.Sprintf("[← %s →]", choice)) + f.bloomAnnotation(bloomPresets[opt.choiceIdx])
		}
		return sDim.Render(fmt.Sprintf("[← %s →]", choice))

	default:
		if opt.enabled {
			return sSuccess.Render("[  ✓  ]")
		}
		return sDim.Render("[  ─  ]")
	}
}

// optHints are shown as a dim hint line when that option is focused.
var optHints = [6]string{
	"Keep lines with at least this many characters",
	"Keep lines with at most this many characters",
	"Drop lines that contain non-printable or non-ASCII bytes",
	"Keep only lines that match a regular expression",
	"Remove duplicate lines — keeps first occurrence",
	"RAM budget for dedup on large files — more RAM = fewer missed duplicates",
}

func (f filterModel) View(width, maxHeight int) string {
	contentWidth := width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// File context header
	fileInfo := sDim.Render("  " + f.fileName)
	if f.fileSize > 0 {
		fileInfo += sDimmer.Render("  " + pipeline.HumanSize(f.fileSize))
	}
	lines := []string{"", fileInfo, ""}

	// renderRow builds one option line + optional hint line when focused.
	renderRow := func(i int, indent bool) []string {
		opt := f.options[i]
		focused := i == f.cursor && !f.inputing

		cur := "   "
		if focused {
			cur = sPrompt.Render("▸") + "  "
		}

		indentPfx := ""
		if indent {
			indentPfx = sDimmer.Render("  └─ ")
		}

		const nameCol = 17
		name := opt.name
		pad := nameCol - len(name)
		if indent {
			pad -= 5
		}
		if pad < 1 {
			pad = 1
		}

		isEnabled := opt.enabled || (opt.cycle && opt.choiceIdx > 0 && f.isDeduplicate())
		var styledName string
		switch {
		case focused:
			styledName = sSelected.Render(name)
		case isEnabled:
			styledName = sSuccess.Render(name)
		case indent && !f.isDeduplicate():
			styledName = sDimmer.Render(name)
		default:
			styledName = sDim.Render(name)
		}

		row := cur + indentPfx + styledName + strings.Repeat(" ", pad) + f.renderOptValue(i)
		if focused && i < len(optHints) {
			hint := sDimmer.Render("       " + optHints[i])
			return []string{row, hint}
		}
		return []string{row}
	}

	appendRows := func(rows []string) {
		lines = append(lines, rows...)
	}

	// Group: LENGTH
	lines = append(lines, renderSectionDivider("LENGTH", contentWidth))
	appendRows(renderRow(0, false))
	appendRows(renderRow(1, false))
	lines = append(lines, "")

	// Group: CONTENT
	lines = append(lines, renderSectionDivider("CONTENT", contentWidth))
	appendRows(renderRow(2, false))
	appendRows(renderRow(3, false))
	lines = append(lines, "")

	// Group: DEDUPLICATION
	lines = append(lines, renderSectionDivider("DEDUPLICATION", contentWidth))
	appendRows(renderRow(4, false))
	appendRows(renderRow(5, true))
	lines = append(lines, "")

	// Output preview + rules summary
	lines = append(lines, sDimmer.Render("  "+strings.Repeat("─", contentWidth-2)))
	outputName := f.buildOutputName(f.fileName)
	lines = append(lines, sDim.Render("  Output  ")+sValue.Render(outputName))

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
		lines = append(lines, sDim.Render("  Rules   ")+sSuccess.Render(strings.Join(rules, ", ")))
	} else {
		lines = append(lines, sWarning.Render("  Rules   ")+sDimmer.Render("none — enable at least one filter"))
	}
	lines = append(lines, "")

	// Start button — MarginLeft keeps all three border lines aligned.
	btnFocused := f.cursor == len(f.options) && !f.inputing
	var btn string
	if btnFocused {
		btn = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 4).
			MarginLeft(2).
			Render("▶   Start Processing")
	} else {
		btn = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Foreground(colorGray).
			Padding(0, 4).
			MarginLeft(2).
			Render("▶   Start Processing")
	}
	lines = append(lines, btn)

	// Validation error
	if f.validationErr != "" {
		lines = append(lines, "")
		lines = append(lines, sError.Render("  ✖  "+f.validationErr))
	}

	return strings.Join(lines, "\n")
}
