package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Merovelous/strainer/internal/pipeline"
)

func (s summaryModel) View(width, maxHeight int) string {
	lines := []string{""}

	if s.cancelled {
		lines = append(lines, sWarning.Render("  ⚡ Cancelled — partial output removed"))
		lines = append(lines, "")
		return strings.Join(lines, "\n")
	}

	// ── Result headline ───────────────────────────────────────────────────
	var retentionPct float64
	if s.linesRead > 0 {
		retentionPct = float64(s.linesKept) / float64(s.linesRead) * 100
	}
	headline := sSuccess.Render("✔  Done") +
		sDim.Render(fmt.Sprintf("   %s elapsed   %.1f%% retained",
			pipeline.FormatDuration(s.elapsed), retentionPct))
	lines = append(lines, "  "+headline)
	lines = append(lines, "")

	// ── Output file box ───────────────────────────────────────────────────
	absOut := s.outputFile
	if abs, err := filepath.Abs(s.outputFile); err == nil {
		absOut = abs
	}
	outBox := sPanelGreen.Width(width - 4).Render(
		sDim.Render("  Saved to\n") + "  " + sSuccess.Render(absOut),
	)
	lines = append(lines, outBox)
	lines = append(lines, "")

	// ── Stats panel ───────────────────────────────────────────────────────
	barWidth := width - 16
	if barWidth < 10 {
		barWidth = 10
	}
	filled := int(retentionPct / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	retBar := sSuccess.Render(strings.Repeat("█", filled)) +
		sDimmer.Render(strings.Repeat("░", barWidth-filled))

	var throughput string
	if s.elapsed.Seconds() > 0 {
		bps := float64(s.bytesRead) / s.elapsed.Seconds()
		throughput = pipeline.HumanSpeed(bps)
	}

	lbl := func(s string) string { return sDim.Render(fmt.Sprintf("  %-14s", s)) }

	panel := lbl("Kept") + sSuccess.Render(pipeline.CommaFmt(s.linesKept)) +
		sDimmer.Render("  of  ") + sValue.Render(pipeline.CommaFmt(s.linesRead)) +
		sDimmer.Render(" read  (") + sValue.Render(fmt.Sprintf("%.1f%%", retentionPct)) + sDimmer.Render(")") + "\n"
	panel += lbl("Dropped") + sError.Render(pipeline.CommaFmt(s.linesDropped)) + "\n"
	panel += lbl("Written") + sValue.Render(pipeline.HumanSize(s.bytesWritten)) +
		sDimmer.Render("  from  ") + sDim.Render(pipeline.HumanSize(s.bytesRead)) + "\n"
	if throughput != "" {
		panel += lbl("Throughput") + sValue.Render(throughput) + "\n"
	}
	panel += "\n  " + retBar + "\n"

	// Filters applied
	var rules []string
	if s.minLen > 0 {
		rules = append(rules, fmt.Sprintf("min=%d", s.minLen))
	}
	if s.maxLen > 0 {
		rules = append(rules, fmt.Sprintf("max=%d", s.maxLen))
	}
	if s.asciiOnly {
		rules = append(rules, "ascii-only")
	}
	if s.regexStr != "" {
		rules = append(rules, "regex")
	}
	if s.deduplicate {
		rules = append(rules, "dedup")
	}
	if len(rules) > 0 {
		panel += "\n" + lbl("Filters") + sSuccess.Render(strings.Join(rules, ", "))
	}

	lines = append(lines, sPanel.Width(width-4).Render(panel))

	return strings.Join(lines, "\n")
}
