package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBg      = lipgloss.Color("#0d0d0f")
	colorBgAlt   = lipgloss.Color("#13131a")
	colorBgHover = lipgloss.Color("#1a1a2e")
	colorFg      = lipgloss.Color("#e2e8f0")
	colorCyan    = lipgloss.Color("#22d3ee")
	colorMagenta = lipgloss.Color("#c084fc")
	colorGreen   = lipgloss.Color("#4ade80")
	colorRed     = lipgloss.Color("#f87171")
	colorYellow  = lipgloss.Color("#fbbf24")
	colorOrange  = lipgloss.Color("#fb923c")
	colorPurple  = lipgloss.Color("#a78bfa")
	colorGray    = lipgloss.Color("#64748b")
	colorDimGray = lipgloss.Color("#1e2533")
	colorWhite   = lipgloss.Color("#f8fafc")
	colorBrightCy = lipgloss.Color("#67e8f9")
	colorBorder  = lipgloss.Color("#2d3748")
	colorMuted   = lipgloss.Color("#4b5563")
)

var (
	sBg = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFg)

	sBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan)

	sHeader = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)

	sSubHeader = lipgloss.NewStyle().
		Foreground(colorMagenta).
		Bold(true)

	sPrompt = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)

	sActive = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)

	sCyan = lipgloss.NewStyle().
		Foreground(colorCyan)

	sMagenta = lipgloss.NewStyle().
		Foreground(colorMagenta)

	sSuccess = lipgloss.NewStyle().
		Foreground(colorGreen)

	sError = lipgloss.NewStyle().
		Foreground(colorRed)

	sWarning = lipgloss.NewStyle().
		Foreground(colorYellow)

	sDim = lipgloss.NewStyle().
		Foreground(colorGray)

	sDimmer = lipgloss.NewStyle().
		Foreground(colorMuted)

	sValue = lipgloss.NewStyle().
		Foreground(colorBrightCy)

	sHighlight = lipgloss.NewStyle().
		Background(colorDimGray).
		Foreground(colorWhite)

	sSelected = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)

	// Full-width row highlight for list views.
	sRowSelected = lipgloss.NewStyle().
		Background(colorBgHover).
		Foreground(colorWhite).
		Bold(true)

	// Section header label inside a separator line.
	sSection = lipgloss.NewStyle().
		Foreground(colorMuted).
		Bold(true)

	// Keyboard hint key (e.g. "Enter", "q").
	sKey = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)

	// Step indicator styles.
	sStepDone   = lipgloss.NewStyle().Foreground(colorGreen)
	sStepActive = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	sStepFuture = lipgloss.NewStyle().Foreground(colorMuted)
	sStepSep    = lipgloss.NewStyle().Foreground(colorMuted)
)

var (
	sPanel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1)

	sPanelGreen = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorGreen).
		Padding(0, 1)

	sPanelYellow = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorYellow).
		Padding(0, 1)

	sPanelRed = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorRed).
		Padding(0, 1)

	sPanelCyan = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(0, 1)
)

var (
	progressChars = []string{"░", "▒", "▓", "█"}
	gradientBar   = []string{"░", "░", "▒", "▒", "▓", "▓", "█", "█"}
	sBarLabel     = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
)
