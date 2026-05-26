package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func newProcessingModel(input, selectedArchiveFile, output string, minLen, maxLen int, asciiOnly bool, isArchive bool) processingModel {
	var fileSize int64
	if info, err := os.Stat(input); err == nil {
		fileSize = info.Size()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return processingModel{
		pipeline: &pipelineModel{
			inputFile:           input,
			selectedArchiveFile: selectedArchiveFile,
			outputFile:          output,
			fileSize:            fileSize,
			minLen:              minLen,
			maxLen:              maxLen,
			asciiOnly:           asciiOnly,
			isArchive:           isArchive,
			ctx:                 ctx,
			cancel:              cancel,
			done:                make(chan struct{}),
			ready:               true,
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
		if pm.pipeline.ready {
			pm.pipeline.start()
		}
		return pm, nil

	case pipeDoneMsg:
		pm.pipeline.status = pipeDone
		return pm, nil

	case pipeErrMsg:
		pm.pipeline.status = pipeError
		return pm, nil

	case metricsTickMsg:
		// Check completion via channel — avoids racing on the status field.
		// err and finishAt are written by the goroutine before close(done),
		// so they are safe to read here after observing the closed channel.
		select {
		case <-pm.pipeline.done:
			if pm.pipeline.err != nil {
				err := pm.pipeline.err
				return pm, func() tea.Msg { return pipeErrMsg{err: err} }
			}
			return pm, func() tea.Msg { return pipeDoneMsg{} }
		default:
		}
		if pm.pipeline.status == pipeRunning {
			now := time.Now()
			ticks := getCPURawTicks()
			if !pm.metrics.prevCPUTime.IsZero() {
				elapsed := now.Sub(pm.metrics.prevCPUTime).Seconds()
				if elapsed > 0 {
					// CPU delta
					delta := ticks - pm.metrics.prevCPUTicks
					pm.metrics.cpuPct = (delta / 100.0 / elapsed / float64(runtime.NumCPU())) * 100.0

					// Rolling EMA speed — alpha=0.3 smooths over ~3 ticks (300ms)
					const alpha = 0.3
					curBR := atomic.LoadInt64(&pm.pipeline.bytesRead)
					curLR := atomic.LoadInt64(&pm.pipeline.linesRead)
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
				pm.metrics.prevBytesRead = atomic.LoadInt64(&pm.pipeline.bytesRead)
				pm.metrics.prevLinesRead = atomic.LoadInt64(&pm.pipeline.linesRead)
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
	header := sHeader.Render("  🔄 PROCESSING")
	fileLine := sDim.Render("  Input: " + pm.pipeline.inputFile)
	lines := []string{"", header, fileLine, ""}

	lr := atomic.LoadInt64(&pm.pipeline.linesRead)
	lk := atomic.LoadInt64(&pm.pipeline.linesKept)
	ld := atomic.LoadInt64(&pm.pipeline.linesDropped)
	br := atomic.LoadInt64(&pm.pipeline.bytesRead)
	bw := atomic.LoadInt64(&pm.pipeline.bytesWritten)

	elapsed := time.Since(pm.metrics.startTime)

	var speed string
	if pm.metrics.currentSpeed > 0 {
		speed = humanSpeed(pm.metrics.currentSpeed)
	}

	var linesPerSec string
	if pm.metrics.currentLPS > 0 {
		linesPerSec = fmt.Sprintf("%.0f lines/s", pm.metrics.currentLPS)
	}

	var statusStr string
	switch pm.pipeline.status {
	case pipeRunning:
		statusStr = sSuccess.Render("● RUNNING")
	case pipeDone:
		if pm.pipeline.err != nil {
			statusStr = sError.Render("✖ ERROR: " + pm.pipeline.err.Error())
		} else {
			statusStr = sSuccess.Render("✔ COMPLETE")
		}
	case pipeError:
		statusStr = sError.Render("✖ ERROR: " + pm.pipeline.err.Error())
	default:
		statusStr = sDim.Render("○ IDLE")
	}

	// ETA (plain files only — % is shown inside the bar itself)
	var etaStr string
	if pm.pipeline.fileSize > 0 && !pm.pipeline.isArchive {
		pct := float64(br) / float64(pm.pipeline.fileSize)
		if pct > 1.0 {
			pct = 1.0
		}
		if pm.metrics.currentSpeed > 0 && pct < 0.999 {
			remaining := float64(pm.pipeline.fileSize-br) / pm.metrics.currentSpeed
			etaStr = "  ETA " + formatDuration(time.Duration(remaining)*time.Second)
		}
	}

	panel := fmt.Sprintf("  %s%s", statusStr, sDim.Render(etaStr))
	if lr > 0 {
		var keptPct string
		if lr > 0 {
			keptPct = fmt.Sprintf(" (%.0f%% kept)", float64(lk)/float64(lr)*100)
		}
		panel += fmt.Sprintf("\n  Lines: %s read  %s kept%s  %s dropped",
			sValue.Render(commaFmt(lr)),
			sSuccess.Render(commaFmt(lk)),
			sDim.Render(keptPct),
			sError.Render(commaFmt(ld)))
	} else {
		panel += fmt.Sprintf("\n  Lines: %s read  %s kept  %s dropped",
			sValue.Render(commaFmt(lr)),
			sSuccess.Render(commaFmt(lk)),
			sError.Render(commaFmt(ld)))
	}
	panel += fmt.Sprintf("\n  Read: %s  Written: %s",
		sValue.Render(humanSize(br)),
		sValue.Render(humanSize(bw)))
	panel += fmt.Sprintf("\n  Speed: %s  %s",
		sValue.Render(speed),
		sDim.Render(linesPerSec))
	panel += fmt.Sprintf("\n  Elapsed: %s", sValue.Render(formatDuration(elapsed)))

	if pm.pipeline.status == pipeRunning && (pm.metrics.cpuPct > 0 || pm.metrics.rssBytes > 0) {
		panel += fmt.Sprintf("\n  CPU: %.1f%%  RAM: %s",
			pm.metrics.cpuPct,
			humanSize(pm.metrics.rssBytes))
	}

	panelBox := sPanel.Width(width - 4).Render(panel)
	lines = append(lines, panelBox)

	lines = append(lines, "")
	bar := renderProgressBar(width-6, pm.pipeline.status == pipeRunning, br, pm.pipeline.fileSize, pm.pipeline.isArchive)
	lines = append(lines, "  "+bar)

	lines = append(lines, "")
	var rules []string
	if pm.pipeline.minLen > 0 {
		rules = append(rules, fmt.Sprintf("min=%d", pm.pipeline.minLen))
	}
	if pm.pipeline.maxLen > 0 {
		rules = append(rules, fmt.Sprintf("max=%d", pm.pipeline.maxLen))
	}
	if pm.pipeline.asciiOnly {
		rules = append(rules, "ascii-only")
	}
	if len(rules) > 0 {
		lines = append(lines, sDim.Render("  Filters: ")+sSuccess.Render(strings.Join(rules, ", ")))
	}

	lines = append(lines, "")
	if pm.pipeline.status == pipeDone || pm.pipeline.status == pipeError {
		lines = append(lines, sDim.Render("  [q] Continue to summary"))
	} else {
		lines = append(lines, sDim.Render("  [q] Cancel"))
	}

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
		bar += sDim.Render(fmt.Sprintf(" %s read", humanSize(bytesRead)))
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

func humanSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	} else if bytesPerSec < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB/s", bytesPerSec/(1024*1024*1024))
}

func commaFmt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatDuration(d time.Duration) string {
	s := int(d.Seconds())
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

// maxArchiveOutput guards against archive bombs — stop writing after 10 GiB.
const maxArchiveOutput = 10 << 30

// --- Pipeline ---

func (p *pipelineModel) start() {
	p.startAt = time.Now()
	p.status = pipeRunning

	go func() {
		var runErr error
		defer func() {
			p.finishAt = time.Now()
			if p.ctx.Err() != nil {
				// User cancelled — delete the partial output file.
				os.Remove(p.outputFile)
				p.err = nil
			} else {
				p.err = runErr
			}
			close(p.done)
		}()

		var reader io.Reader
		if p.isArchive {
			args := []string{"x", p.inputFile, "-so", "-mmt=on"}
			if p.selectedArchiveFile != "" {
				args = append(args, p.selectedArchiveFile)
			}
			// CommandContext kills the 7z process when ctx is cancelled.
			cmd := exec.CommandContext(p.ctx, sevenZipBin, args...)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				runErr = err
				return
			}
			if err := cmd.Start(); err != nil {
				runErr = err
				return
			}
			reader = stdout
			defer cmd.Wait()
		} else {
			f, err := os.Open(p.inputFile)
			if err != nil {
				runErr = err
				return
			}
			defer f.Close()
			reader = f
		}

		cr := &atomicCounterReader{r: reader, bytesRead: &p.bytesRead}

		outFile, err := os.OpenFile(p.outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			runErr = err
			return
		}
		defer outFile.Close()

		writer := bufio.NewWriterSize(outFile, 256*1024)
		scanner := bufio.NewScanner(cr)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			select {
			case <-p.ctx.Done():
				return
			default:
			}
			if p.isArchive && atomic.LoadInt64(&p.bytesWritten) > maxArchiveOutput {
				runErr = fmt.Errorf("output limit reached (%s) — possible archive bomb", humanSize(maxArchiveOutput))
				return
			}
			atomic.AddInt64(&p.linesRead, 1)
			line := scanner.Bytes()
			// Strip Windows CRLF — Scanner splits on \n but leaves \r on the token.
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if !p.filterLine(line) {
				atomic.AddInt64(&p.linesDropped, 1)
				continue
			}
			atomic.AddInt64(&p.linesKept, 1)
			writer.Write(line)
			writer.WriteByte('\n')
			atomic.AddInt64(&p.bytesWritten, int64(len(line))+1)
		}

		if err := scanner.Err(); err != nil {
			if p.ctx.Err() == nil {
				runErr = err
			}
			return
		}

		writer.Flush()
	}()
}

// filterLine works on []byte directly — no string allocation
func (p *pipelineModel) filterLine(line []byte) bool {
	ll := len(line)
	if p.minLen > 0 && ll < p.minLen {
		return false
	}
	if p.maxLen > 0 && ll > p.maxLen {
		return false
	}
	if p.asciiOnly {
		for _, b := range line {
			if b < 0x20 || b > 0x7E {
				return false
			}
		}
	}
	return true
}

// atomicCounterReader wraps io.Reader, atomically counting bytes read
type atomicCounterReader struct {
	r         io.Reader
	bytesRead *int64
}

func (cr *atomicCounterReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	if n > 0 {
		atomic.AddInt64(cr.bytesRead, int64(n))
	}
	return n, err
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return string(out), err
}
