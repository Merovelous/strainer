package main

import "github.com/charmbracelet/lipgloss"

// Dark hacker theme colors
var (
	colorBg       = lipgloss.Color("#0a0a0a")
	colorBgAlt    = lipgloss.Color("#111111")
	colorFg       = lipgloss.Color("#e0e0e0")
	colorCyan     = lipgloss.Color("#00d4ff")
	colorMagenta  = lipgloss.Color("#d946ef")
	colorGreen    = lipgloss.Color("#22c55e")
	colorRed      = lipgloss.Color("#ef4444")
	colorYellow   = lipgloss.Color("#eab308")
	colorOrange   = lipgloss.Color("#f97316")
	colorPurple   = lipgloss.Color("#a855f7")
	colorGray     = lipgloss.Color("#6b7280")
	colorDimGray  = lipgloss.Color("#374151")
	colorWhite    = lipgloss.Color("#ffffff")
	colorBrightCy = lipgloss.Color("#67e8f9")
)

// Base styles
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
		Foreground(colorGreen).
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
		Foreground(colorDimGray)

	sValue = lipgloss.NewStyle().
		Foreground(colorBrightCy)

	sHighlight = lipgloss.NewStyle().
		Background(colorDimGray).
		Foreground(colorWhite)

	sSelected = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)
)

// Component-specific styles
var (
	sPanel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
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
)

// Progress bar characters
var (
	progressChars = []string{"░", "▒", "▓", "█"}
	gradientBar   = []string{"░", "░", "▒", "▒", "▓", "▓", "█", "█"}
)
