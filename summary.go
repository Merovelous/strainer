package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (s summaryModel) View(width, maxHeight int) string {
	var header string
	if s.cancelled {
		header = sWarning.Render("  ⚡ CANCELLED")
	} else {
		header = sHeader.Render("  ✅ SUMMARY")
	}
	lines := []string{"", header, ""}

	if s.cancelled {
		lines = append(lines, sDim.Render("  Partial output file removed."))
		lines = append(lines, "")
	} else {
		// Output path — show absolute path so user can find the file
		absOut := s.outputFile
		if abs, err := filepath.Abs(s.outputFile); err == nil {
			absOut = abs
		}
		outBox := sPanelGreen.Width(width - 4).Render(
			sSubHeader.Render("  Output file\n") +
				"  " + sSuccess.Render(absOut),
		)
		lines = append(lines, outBox)
		lines = append(lines, "")
	}

	// Two-column stats panel
	var throughput string
	if s.elapsed.Seconds() > 0 {
		bps := float64(s.bytesRead) / s.elapsed.Seconds()
		throughput = humanSpeed(bps)
	}

	var retentionBar string
	var retentionPct float64
	if s.linesRead > 0 {
		retentionPct = float64(s.linesKept) / float64(s.linesRead) * 100
		barWidth := width - 24
		if barWidth < 10 {
			barWidth = 10
		}
		filled := int(retentionPct / 100 * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		retentionBar = sSuccess.Render(strings.Repeat("█", filled)) +
			sDimmer.Render(strings.Repeat("░", barWidth-filled)) +
			sValue.Render(fmt.Sprintf("  %.1f%%", retentionPct))
	}

	panel := ""
	panel += fmt.Sprintf("  %-18s %s\n", sSubHeader.Render("Input:"), sDim.Render(filepath.Base(s.inputFile)))
	panel += fmt.Sprintf("\n  %-18s %s    %s\n", sSubHeader.Render("Lines read:"), sValue.Render(commaFmt(s.linesRead)), sDim.Render("Data read:   "+humanSize(s.bytesRead)))
	panel += fmt.Sprintf("  %-18s %s    %s\n", sSubHeader.Render("Lines kept:"), sSuccess.Render(commaFmt(s.linesKept)), sDim.Render("Data written: "+humanSize(s.bytesWritten)))
	panel += fmt.Sprintf("  %-18s %s    %s\n", sSubHeader.Render("Lines dropped:"), sError.Render(commaFmt(s.linesDropped)), sDim.Render("Elapsed:      "+formatDuration(s.elapsed)))
	if throughput != "" {
		panel += fmt.Sprintf("  %-18s %s\n", sSubHeader.Render("Throughput:"), sValue.Render(throughput))
	}

	if retentionBar != "" {
		panel += fmt.Sprintf("\n  %s\n  %s", sSubHeader.Render("Retention"), retentionBar)
	}

	lines = append(lines, sPanel.Width(width-4).Render(panel))

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
		rules = append(rules, "regex="+s.regexStr)
	}
	if s.deduplicate {
		rules = append(rules, "dedup")
	}
	if len(rules) > 0 {
		lines = append(lines, "")
		lines = append(lines, sDim.Render("  Filters: ")+sSuccess.Render(strings.Join(rules, ", ")))
	}

	lines = append(lines, "")
	lines = append(lines, sDim.Render("  [q] Exit  [r] Process another file"))

	return strings.Join(lines, "\n")
}
