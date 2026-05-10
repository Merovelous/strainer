package main

import (
	"fmt"
	"strings"
)

func (s summaryModel) View(width, maxHeight int) string {
	header := sHeader.Render("  ✅ SUMMARY")
	lines := []string{"", header, ""}

	// Stats panel
	panel := fmt.Sprintf("  %s %s", sSubHeader.Render("Input:"), sValue.Render(s.inputFile))
	panel += fmt.Sprintf("\n  %s %s", sSubHeader.Render("Output:"), sSuccess.Render(s.outputFile))
	panel += "\n"
	panel += fmt.Sprintf("\n  %s %s lines", sSubHeader.Render("Read:"), sValue.Render(commaFmt(s.linesRead)))
	panel += fmt.Sprintf("\n  %s %s lines", sSubHeader.Render("Kept:"), sSuccess.Render(commaFmt(s.linesKept)))
	panel += fmt.Sprintf("\n  %s %s lines", sSubHeader.Render("Dropped:"), sError.Render(commaFmt(s.linesDropped)))

	if s.linesRead > 0 {
		pct := float64(s.linesKept) / float64(s.linesRead) * 100
		panel += fmt.Sprintf("\n  %s %.1f%%", sSubHeader.Render("Retention:"), pct)
	}

	panel += fmt.Sprintf("\n  %s %s", sSubHeader.Render("Data read:"), sValue.Render(humanSize(s.bytesRead)))
	panel += fmt.Sprintf("\n  %s %s", sSubHeader.Render("Data written:"), sSuccess.Render(humanSize(s.bytesWritten)))
	panel += fmt.Sprintf("\n  %s %s", sSubHeader.Render("Elapsed:"), sValue.Render(formatDuration(s.elapsed)))

	panelBox := sPanelGreen.Width(width - 4).Render(panel)
	lines = append(lines, panelBox)

	// Rules applied
	lines = append(lines, "")
	var rules []string
	if s.minLen > 0 {
		rules = append(rules, fmt.Sprintf("min length: %d", s.minLen))
	}
	if s.maxLen > 0 {
		rules = append(rules, fmt.Sprintf("max length: %d", s.maxLen))
	}
	if s.asciiOnly {
		rules = append(rules, "ASCII only [\\x20-\\x7E]")
	}
	if len(rules) > 0 {
		rulesStr := sDim.Render("  Rules applied: ") + sSuccess.Render(strings.Join(rules, ", "))
		lines = append(lines, rulesStr)
	}

	// Footer
	lines = append(lines, "")
	lines = append(lines, sDim.Render("  [q] Exit  [r] Restart with new file"))

	return strings.Join(lines, "\n")
}
