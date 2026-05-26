package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Merovelous/strainer/internal/pipeline"
)

// Run creates the TUI appModel and starts the BubbleTea program.
func Run(version string) error {
	m := newAppModel(version)
	p := tea.NewProgram(m, tea.WithAltScreen())
	globalTeaProgram = p
	_, err := p.Run()
	return err
}

func newAppModel(version string) appModel {
	wd, _ := os.Getwd()
	return appModel{
		version:    version,
		state:      stateBrowser,
		workingDir: wd,
		browser:    newBrowserModel(wd),
	}
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		m.browser.Init(),
		tea.WindowSize(),
	)
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.browser.windowHeight = msg.Height
		m.archivePicker.windowHeight = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.String() == "q" {
			switch m.state {
			case stateBrowser:
				m.quitting = true
				return m, tea.Quit
			case stateArchivePicker:
				m.state = stateBrowser
				return m, nil
			case stateFilters:
				m.state = stateBrowser
				return m, nil
			case stateOverwriteConfirm:
				m.state = stateFilters
				return m, nil
			case stateProcessing:
				p := m.processing.pipeline
				p.Cancel()
				m.state = stateSummary
				m.summary = summaryModel{
					inputFile:    m.inputFile,
					outputFile:   m.outputFile,
					linesRead:    atomic.LoadInt64(&p.LinesRead),
					linesKept:    atomic.LoadInt64(&p.LinesKept),
					linesDropped: atomic.LoadInt64(&p.LinesDropped),
					bytesRead:    atomic.LoadInt64(&p.BytesRead),
					bytesWritten: atomic.LoadInt64(&p.BytesWritten),
					elapsed:      time.Since(p.StartAt),
					minLen:       m.minLen,
					maxLen:       m.maxLen,
					asciiOnly:    m.asciiOnly,
					regexStr:     m.regexStr,
					deduplicate:  m.deduplicate,
					ready:        true,
					cancelled:    true,
				}
				return m, nil
			case stateSummary:
				m.quitting = true
				return m, tea.Quit
			}
		}
		if msg.String() == "esc" && m.state == stateFilters {
			m.state = stateBrowser
			return m, nil
		}

	case browserSelectMsg:
		m.inputFile = msg.path
		if info, err := os.Stat(msg.path); err == nil {
			m.inputFileSize = info.Size()
		}
		if msg.isArchive {
			m.isArchive = true
			m.state = stateArchivePicker
			ap, cmd := newArchivePickerModel(msg.path)
			m.archivePicker = ap
			return m, cmd
		}
		m.isArchive = false
		if pipeline.IsLikelyBinary(msg.path) {
			m.browser.err = fmt.Errorf("%s appears to be a binary file — select a plain text wordlist", filepath.Base(msg.path))
			m.state = stateBrowser
			return m, nil
		}
		m.state = stateFilters
		m.filters = newFilterModel(filepath.Base(msg.path), m.inputFileSize)
		return m, nil

	case archiveSelectMsg:
		m.selectedArchiveFile = msg.file
		m.state = stateFilters
		m.filters = newFilterModel(msg.file, m.inputFileSize)
		return m, nil

	case filterConfirmMsg:
		outputName := m.filters.buildOutputName(m.inputFile)
		m.outputFile = outputName
		m.minLen = m.filters.getMinLen()
		m.maxLen = m.filters.getMaxLen()
		m.asciiOnly = m.filters.isASCIIOnly()
		m.regexStr = m.filters.getRegexStr()
		m.deduplicate = m.filters.isDeduplicate()
		m.bloomSize = m.filters.getBloomSize()

		if _, err := os.Stat(outputName); err == nil {
			m.state = stateOverwriteConfirm
			return m, nil
		}

		var re *regexp.Regexp
		if m.regexStr != "" {
			re, _ = regexp.Compile(m.regexStr)
		}
		m.state = stateProcessing
		pm := newProcessingModel(m.inputFile, m.selectedArchiveFile, outputName, m.minLen, m.maxLen, m.asciiOnly, m.isArchive, re, m.deduplicate, m.bloomSize)
		m.processing = pm
		return m, m.processing.Init()

	case pipeDoneMsg:
		if m.state != stateProcessing {
			return m, nil
		}
		m.state = stateSummary
		p := m.processing.pipeline
		m.summary = summaryModel{
			inputFile:    m.inputFile,
			outputFile:   m.outputFile,
			linesRead:    p.LinesRead,
			linesKept:    p.LinesKept,
			linesDropped: p.LinesDropped,
			bytesRead:    p.BytesRead,
			bytesWritten: p.BytesWritten,
			elapsed:      p.FinishAt.Sub(p.StartAt),
			minLen:       m.minLen,
			maxLen:       m.maxLen,
			asciiOnly:    m.asciiOnly,
			regexStr:     m.regexStr,
			deduplicate:  m.deduplicate,
			ready:        true,
		}
		return m, nil

	case pipeErrMsg:
		if m.state != stateProcessing {
			return m, nil
		}
		m.state = stateSummary
		p := m.processing.pipeline
		m.summary = summaryModel{
			inputFile:    m.inputFile,
			outputFile:   m.outputFile,
			linesRead:    p.LinesRead,
			linesKept:    p.LinesKept,
			linesDropped: p.LinesDropped,
			bytesRead:    p.BytesRead,
			bytesWritten: p.BytesWritten,
			elapsed:      time.Since(p.StartAt),
			minLen:       m.minLen,
			maxLen:       m.maxLen,
			asciiOnly:    m.asciiOnly,
			regexStr:     m.regexStr,
			deduplicate:  m.deduplicate,
			ready:        true,
		}
		return m, nil

	case metricsTickMsg:
		if m.state == stateProcessing {
			var cmd tea.Cmd
			m.processing, cmd = m.processing.Update(msg, globalTeaProgram)
			return m, cmd
		}
		return m, nil
	}

	switch m.state {
	case stateBrowser:
		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd
	case stateArchivePicker:
		var cmd tea.Cmd
		m.archivePicker, cmd = m.archivePicker.Update(msg)
		return m, cmd
	case stateFilters:
		var cmd tea.Cmd
		m.filters, cmd = m.filters.Update(msg)
		return m, cmd
	case stateProcessing:
		var cmd tea.Cmd
		m.processing, cmd = m.processing.Update(msg, globalTeaProgram)
		return m, cmd
	case stateOverwriteConfirm:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "y":
				var re *regexp.Regexp
				if m.regexStr != "" {
					re, _ = regexp.Compile(m.regexStr)
				}
				m.state = stateProcessing
				pm := newProcessingModel(m.inputFile, m.selectedArchiveFile, m.outputFile, m.minLen, m.maxLen, m.asciiOnly, m.isArchive, re, m.deduplicate, m.bloomSize)
				m.processing = pm
				return m, m.processing.Init()
			case "n", "esc":
				m.state = stateFilters
				return m, nil
			}
		}

	case stateSummary:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "r" {
			m = newAppModel(m.version)
			return m, m.Init()
		}
	}

	return m, nil
}

func renderSteps(state viewState) string {
	type step struct {
		label string
		idx   int
	}
	steps := []step{
		{"Select", 0},
		{"Filters", 1},
		{"Process", 2},
		{"Done", 3},
	}
	current := func() int {
		switch state {
		case stateBrowser, stateArchivePicker:
			return 0
		case stateFilters, stateOverwriteConfirm:
			return 1
		case stateProcessing:
			return 2
		case stateSummary:
			return 3
		}
		return 0
	}()
	var parts []string
	for _, s := range steps {
		var rendered string
		switch {
		case s.idx < current:
			rendered = sStepDone.Render("✓ " + s.label)
		case s.idx == current:
			rendered = sStepActive.Render("● " + s.label)
		default:
			rendered = sStepFuture.Render("○ " + s.label)
		}
		parts = append(parts, rendered)
	}
	sep := sStepSep.Render("  ›  ")
	return strings.Join(parts, sep)
}

func renderHints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, sKey.Render(pairs[i])+sDim.Render(" "+pairs[i+1]))
	}
	return strings.Join(parts, sDimmer.Render("  ·  "))
}

func (m appModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	h := m.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	// Title bar: logo left, step breadcrumb right.
	logo := sHeader.Render("⚡ strainer") + sDim.Render(" v"+m.version)
	steps := renderSteps(m.state)
	stepsWidth := lipgloss.Width(steps)
	logoWidth := lipgloss.Width(logo)
	gap := w - logoWidth - stepsWidth - 4
	if gap < 2 {
		gap = 2
	}
	titleContent := "  " + logo + strings.Repeat(" ", gap) + steps + "  "
	titleBar := lipgloss.NewStyle().
		Width(w).
		Background(colorBgAlt).
		Render(titleContent)

	var content string
	switch m.state {
	case stateBrowser:
		content = m.browser.View(w, h-4)
	case stateArchivePicker:
		content = m.archivePicker.View(w, h-4)
	case stateFilters:
		content = m.filters.View(w, h-4)
	case stateOverwriteConfirm:
		absOut := m.outputFile
		if abs, err := filepath.Abs(m.outputFile); err == nil {
			absOut = abs
		}
		header := sWarning.Render("  ⚠  Output File Exists")
		detail := "\n\n  " + sDim.Render("Path: ") + sWarning.Render(absOut) +
			"\n\n  Overwrite the existing file?"
		box := sPanelYellow.Width(w - 4).Render(header + detail)
		yesHint := sSuccess.Render("y") + sDim.Render(" overwrite")
		noHint := sError.Render("n") + sDim.Render(" keep existing")
		content = "\n" + box + "\n\n  " + yesHint + "   " + noHint
	case stateProcessing:
		content = m.processing.View(w, h-4)
	case stateSummary:
		content = m.summary.View(w, h-4)
	}

	// Footer bar: contextual key hints.
	var hints string
	switch m.state {
	case stateBrowser:
		hints = renderHints("↑↓", "navigate", "Enter", "select", "⌫", "parent dir", "q", "quit")
	case stateArchivePicker:
		hints = renderHints("↑↓", "navigate", "Enter", "select file", "q", "back")
	case stateFilters:
		if m.filters.inputing {
			hints = renderHints("Type", "enter value", "Enter", "confirm", "Esc", "cancel")
		} else {
			hints = renderHints("↑↓", "navigate", "Space", "toggle", "←→", "adjust", "Enter", "edit", "q", "back")
		}
	case stateOverwriteConfirm:
		hints = renderHints("y", "overwrite", "n / Esc", "cancel")
	case stateProcessing:
		hints = renderHints("q", "cancel processing")
	case stateSummary:
		hints = renderHints("r", "process another file", "q", "exit")
	}
	footerBar := lipgloss.NewStyle().
		Width(w).
		Background(colorBgAlt).
		Padding(0, 1).
		Render("  " + hints)

	return fmt.Sprintf("%s\n%s\n%s", titleBar, content, footerBar)
}
