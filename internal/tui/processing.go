package tui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Merovelous/strainer/internal/pipeline"
	tea "github.com/charmbracelet/bubbletea"
)

// globalTeaProgram is set by Run() so the pipeline goroutine can send messages.
var globalTeaProgram *tea.Program

func newProcessingModel(input, selectedArchiveFile, output string, minLen, maxLen int, asciiOnly bool, isArchive bool, regex *regexp.Regexp, deduplicate bool, bloomSize int64) processingModel {
	var fileSize int64
	if info, err := os.Stat(input); err == nil {
		fileSize = info.Size()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return processingModel{
		pipeline: &pipeline.Model{
			InputFile:           input,
			SelectedArchiveFile: selectedArchiveFile,
			OutputFile:          output,
			FileSize:            fileSize,
			MinLen:              minLen,
			MaxLen:              maxLen,
			ASCIIOnly:           asciiOnly,
			IsArchive:           isArchive,
			Regex:               regex,
			Deduplicate:         deduplicate,
			BloomSize:           bloomSize,
			Ctx:                 ctx,
			Cancel:              cancel,
			Done:                make(chan struct{}),
			Ready:               true,
		},
		metrics: metricsModel{
			startTime: time.Now(),
		},
	}
}

func (pm processingModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return pipeReadyMsg{} },
		tea.Every(100*time.Millisecond, func(t time.Time) tea.Msg {
			return metricsTickMsg{}
		}),
	)
}

func (pm processingModel) Update(msg tea.Msg, teaProgram *tea.Program) (processingModel, tea.Cmd) {
	switch msg.(type) {
	case pipeReadyMsg:
		if pm.pipeline.Ready {
			pm.pipeline.Start()
		}
		return pm, nil

	case pipeDoneMsg:
		pm.pipeline.Status = pipeline.Done
		return pm, nil

	case pipeErrMsg:
		pm.pipeline.Status = pipeline.Error
		return pm, nil

	case metricsTickMsg:
		// Check completion via channel — avoids racing on the status field.
		// Err and FinishAt are written by the goroutine before close(Done),
		// so they are safe to read here after observing the closed channel.
		select {
		case <-pm.pipeline.Done:
			if pm.pipeline.Err != nil {
				err := pm.pipeline.Err
				return pm, func() tea.Msg { return pipeErrMsg{err: err} }
			}
			return pm, func() tea.Msg { return pipeDoneMsg{} }
		default:
		}
		if pm.pipeline.Status == pipeline.Running {
			now := time.Now()
			ticks := getCPURawTicks()
			if !pm.metrics.prevCPUTime.IsZero() {
				elapsed := now.Sub(pm.metrics.prevCPUTime).Seconds()
				if elapsed > 0 {
					// CPU delta
					delta := ticks - pm.metrics.prevCPUTicks
					pm.metrics.cpuPct = (delta / cpuTicksPerSec() / elapsed / float64(runtime.NumCPU())) * 100.0

					// Rolling EMA speed — alpha=0.3 smooths over ~3 ticks (300ms)
					const alpha = 0.3
					curBR := atomic.LoadInt64(&pm.pipeline.BytesRead)
					curLR := atomic.LoadInt64(&pm.pipeline.LinesRead)
					bSample := float64(curBR-pm.metrics.prevBytesRead) / elapsed
					lSample := float64(curLR-pm.metrics.prevLinesRead) / elapsed
					if pm.metrics.currentSpeed == 0 {
						pm.metrics.currentSpeed = bSample
						pm.metrics.currentLPS = lSample
					} else {
						pm.metrics.currentSpeed = alpha*bSample + (1-alpha)*pm.metrics.currentSpeed
						pm.metrics.currentLPS = alpha*lSample + (1-alpha)*pm.metrics.currentLPS
					}
					pm.metrics.prevBytesRead = curBR
					pm.metrics.prevLinesRead = curLR
				}
			} else {
				// First tick — seed prev values so next tick gets a clean delta
				pm.metrics.prevBytesRead = atomic.LoadInt64(&pm.pipeline.BytesRead)
				pm.metrics.prevLinesRead = atomic.LoadInt64(&pm.pipeline.LinesRead)
			}
			pm.metrics.prevCPUTicks = ticks
			pm.metrics.prevCPUTime = now
			pm.metrics.rssBytes = getRSSBytes()
		}
		return pm, tea.Every(100*time.Millisecond, func(t time.Time) tea.Msg {
			return metricsTickMsg{}
		})
	}
	return pm, nil
}

func (pm processingModel) View(width, maxHeight int) string {
	lr := atomic.LoadInt64(&pm.pipeline.LinesRead)
	lk := atomic.LoadInt64(&pm.pipeline.LinesKept)
	ld := atomic.LoadInt64(&pm.pipeline.LinesDropped)
	br := atomic.LoadInt64(&pm.pipeline.BytesRead)
	bw := atomic.LoadInt64(&pm.pipeline.BytesWritten)
	elapsed := time.Since(pm.metrics.startTime)
	barWidth := width - 6

	lines := []string{""}

	// ── Status line ───────────────────────────────────────────────────────
	var statusStr string
	switch pm.pipeline.Status {
	case pipeline.Running:
		statusStr = sSuccess.Render("● Running")
	case pipeline.Done:
		if pm.pipeline.Err != nil {
			statusStr = sError.Render("✖ Error: " + pm.pipeline.Err.Error())
		} else {
			statusStr = sSuccess.Render("✔ Complete")
		}
	case pipeline.Error:
		statusStr = sError.Render("✖ Error: " + pm.pipeline.Err.Error())
	default:
		statusStr = sDim.Render("○ Starting...")
	}

	var etaStr string
	if pm.pipeline.FileSize > 0 && !pm.pipeline.IsArchive && pm.metrics.currentSpeed > 0 {
		pct := float64(br) / float64(pm.pipeline.FileSize)
		if pct < 0.999 {
			remaining := float64(pm.pipeline.FileSize-br) / pm.metrics.currentSpeed
			etaStr = sDim.Render("   ETA " + pipeline.FormatDuration(time.Duration(remaining)*time.Second))
		}
	}
	lines = append(lines, "  "+statusStr+etaStr)

	// ── Progress bar ──────────────────────────────────────────────────────
	lines = append(lines, "")
	bar := renderProgressBar(barWidth, pm.pipeline.Status == pipeline.Running, br, pm.pipeline.FileSize, pm.pipeline.IsArchive)
	lines = append(lines, "  "+bar)
	lines = append(lines, "")

	// ── Metrics panel ─────────────────────────────────────────────────────
	var keptPct string
	if lr > 0 {
		keptPct = sDimmer.Render(fmt.Sprintf("  %.0f%% kept", float64(lk)/float64(lr)*100))
	}

	var speedStr, lpsStr string
	if pm.metrics.currentSpeed > 0 {
		speedStr = pipeline.HumanSpeed(pm.metrics.currentSpeed)
	} else {
		speedStr = "─"
	}
	if pm.metrics.currentLPS > 0 {
		lpsStr = fmt.Sprintf("%.0f lines/s", pm.metrics.currentLPS)
	}

	col := func(label, value string) string {
		return sDim.Render(fmt.Sprintf("  %-10s", label)) + sValue.Render(value)
	}

	panel := col("Read", pipeline.CommaFmt(lr)+" lines") + keptPct + "\n"
	panel += col("Kept", pipeline.CommaFmt(lk)+" lines") +
		sDimmer.Render(fmt.Sprintf("  %s dropped", pipeline.CommaFmt(ld))) + "\n"
	panel += col("Data", pipeline.HumanSize(br)+" read") +
		sDimmer.Render("   "+pipeline.HumanSize(bw)+" written") + "\n"
	panel += col("Speed", speedStr) + sDimmer.Render("   "+lpsStr) + "\n"
	panel += col("Elapsed", pipeline.FormatDuration(elapsed))
	if pm.pipeline.Status == pipeline.Running && (pm.metrics.cpuPct > 0 || pm.metrics.rssBytes > 0) {
		panel += sDimmer.Render(fmt.Sprintf("   CPU %.1f%%   RAM %s",
			pm.metrics.cpuPct, pipeline.HumanSize(pm.metrics.rssBytes)))
	}

	lines = append(lines, sPanel.Width(width-4).Render(panel))

	return strings.Join(lines, "\n")
}

func renderProgressBar(width int, running bool, bytesRead, fileSize int64, isArchive bool) string {
	if width < 10 {
		width = 10
	}

	// Calculate progress percentage (only for plain files, not archives)
	var pct float64
	if fileSize > 0 && !isArchive {
		pct = float64(bytesRead) / float64(fileSize)
		if pct > 1.0 {
			pct = 1.0
		}
	}

	if !running && pct >= 1.0 {
		return barWithLabel(width, width, " 100% ")
	}

	if !running {
		return sDimmer.Render(strings.Repeat("░", width))
	}

	// Archive: animated indeterminate bar — lit segment sweeps left to right
	if isArchive {
		segWidth := width / 3
		if segWidth < 4 {
			segWidth = 4
		}
		tick := int(time.Now().UnixMilli() / 100)
		start := tick % (width + segWidth) - segWidth
		bar := ""
		for i := 0; i < width; i++ {
			if i >= start && i < start+segWidth {
				idx := i % len(gradientBar)
				bar += sCyan.Render(gradientBar[idx])
			} else {
				bar += sDimmer.Render("░")
			}
		}
		bar += sDim.Render(fmt.Sprintf(" %s read", pipeline.HumanSize(bytesRead)))
		return bar
	}

	// Plain file: determinate progress with centered label
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	return barWithLabel(width, filled, fmt.Sprintf(" %.1f%% ", pct*100))
}

// barWithLabel renders a progress bar of the given width with `filled` cyan
// cells, and overlays label centered inside it.
func barWithLabel(width, filled int, label string) string {
	labelLen := len(label)
	labelStart := (width - labelLen) / 2
	if labelStart < 0 {
		labelStart = 0
	}
	var buf strings.Builder
	for i := 0; i < width; i++ {
		li := i - labelStart
		if li >= 0 && li < labelLen {
			buf.WriteString(sBarLabel.Render(string(label[li])))
		} else if i < filled {
			buf.WriteString(sCyan.Render("█"))
		} else {
			buf.WriteString(sDimmer.Render("░"))
		}
	}
	return buf.String()
}
