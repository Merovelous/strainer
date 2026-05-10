package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var asciiRegex = regexp.MustCompile(`^[\x20-\x7E]+$`)

func newProcessingModel(input, output string, minLen, maxLen int, asciiOnly bool, isArchive bool) processingModel {
	return processingModel{
		pipeline: pipelineModel{
			inputFile:  input,
			outputFile: output,
			minLen:     minLen,
			maxLen:     maxLen,
			asciiOnly:  asciiOnly,
			isArchive:  isArchive,
			ready:      true,
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
	switch msg := msg.(type) {
	case pipeReadyMsg:
		if pm.pipeline.ready {
			pm.pipeline.start(teaProgram)
		}
		return pm, nil

	case pipeLineMsg:
		atomic.AddInt64(&pm.metrics.linesRead, 1)
		if msg.kept {
			atomic.AddInt64(&pm.metrics.linesKept, 1)
		} else {
			atomic.AddInt64(&pm.metrics.linesDropped, 1)
		}
		return pm, nil

	case pipeBytesReadMsg:
		atomic.AddInt64(&pm.metrics.bytesRead, msg.n)
		return pm, nil

	case pipeBytesWrittenMsg:
		atomic.AddInt64(&pm.metrics.bytesWritten, msg.n)
		return pm, nil

	case pipeDoneMsg:
		pm.pipeline.status = pipeDone
		pm.pipeline.finishAt = time.Now()
		return pm, nil

	case pipeErrMsg:
		pm.pipeline.status = pipeError
		pm.pipeline.err = msg.err
		pm.pipeline.finishAt = time.Now()
		return pm, nil

	case metricsTickMsg:
		if pm.pipeline.status == pipeRunning {
			pm.metrics.cpuPct = getCPUTimePercent()
			pm.metrics.rssBytes = getRSSBytes()
			pm.metrics.ioReadBytes, pm.metrics.ioWriteBytes = getIOBytes()
			return pm, tea.Every(100*time.Millisecond, func(t time.Time) tea.Msg {
				return metricsTickMsg{}
			})
		}
		return pm, nil

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			pm.pipeline.status = pipeDone
			return pm, tea.Quit
		}
	}
	return pm, nil
}

type (
	pipeBytesReadMsg    struct{ n int64 }
	pipeBytesWrittenMsg struct{ n int64 }
)

func (pm processingModel) View(width, maxHeight int) string {
	header := sHeader.Render("  🔄 PROCESSING")
	fileLine := sDim.Render("  Input: " + pm.pipeline.inputFile)
	lines := []string{"", header, fileLine, ""}

	lr := atomic.LoadInt64(&pm.metrics.linesRead)
	lk := atomic.LoadInt64(&pm.metrics.linesKept)
	ld := atomic.LoadInt64(&pm.metrics.linesDropped)
	br := atomic.LoadInt64(&pm.metrics.bytesRead)
	bw := atomic.LoadInt64(&pm.metrics.bytesWritten)

	elapsed := time.Since(pm.metrics.startTime)
	var speed string
	if elapsed.Seconds() > 0 {
		bytesPerSec := float64(br) / elapsed.Seconds()
		speed = humanSpeed(bytesPerSec)
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

	panel := fmt.Sprintf("  %s", statusStr)
	panel += fmt.Sprintf("\n  Lines: %s read  %s kept  %s dropped",
		sValue.Render(commaFmt(lr)),
		sSuccess.Render(commaFmt(lk)),
		sError.Render(commaFmt(ld)))
	panel += fmt.Sprintf("\n  Read: %s  Written: %s  Speed: %s",
		sValue.Render(humanSize(br)),
		sValue.Render(humanSize(bw)),
		sValue.Render(speed))
	panel += fmt.Sprintf("\n  Elapsed: %s", sValue.Render(formatDuration(elapsed)))

	if pm.pipeline.status == pipeRunning {
		panel += fmt.Sprintf("\n  CPU: %.1f%%  RAM: %s",
			pm.metrics.cpuPct,
			humanSize(pm.metrics.rssBytes))
		if pm.metrics.ioReadBytes > 0 || pm.metrics.ioWriteBytes > 0 {
			panel += fmt.Sprintf("  IO: R %s / W %s",
				humanSize(pm.metrics.ioReadBytes),
				humanSize(pm.metrics.ioWriteBytes))
		}
	}

	panelBox := sPanel.Width(width - 4).Render(panel)
	lines = append(lines, panelBox)

	lines = append(lines, "")
	bar := renderProgressBar(width-6, pm.pipeline.status == pipeRunning)
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

func renderProgressBar(width int, running bool) string {
	if width < 10 {
		width = 10
	}
	if running {
		filled := width / 2
		bar := ""
		for i := 0; i < width; i++ {
			if i < filled {
				idx := i % len(gradientBar)
				bar += sCyan.Render(gradientBar[idx])
			} else {
				bar += sDimmer.Render("░")
			}
		}
		return bar
	}
	return sCyan.Render(strings.Repeat("█", width))
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

// --- Pipeline ---

func (p *pipelineModel) start(teaProgram *tea.Program) {
	p.status = pipeRunning

	lineChan := make(chan string, 4096)
	resultChan := make(chan filterResult, 4096)

	// Reader goroutine
	go func() {
		var reader io.Reader
		if p.isArchive {
			cmd := exec.Command("7z", "x", p.inputFile, "-so", "-mmt=on")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				teaProgram.Send(pipeErrMsg{err: err})
				close(lineChan)
				return
			}
			if err := cmd.Start(); err != nil {
				teaProgram.Send(pipeErrMsg{err: err})
				close(lineChan)
				return
			}
			reader = stdout
			defer cmd.Wait()
		} else {
			f, err := os.Open(p.inputFile)
			if err != nil {
				teaProgram.Send(pipeErrMsg{err: err})
				close(lineChan)
				return
			}
			defer f.Close()
			reader = f
		}

		cr := &countingReader{r: reader, teaProgram: teaProgram}
		scanner := bufio.NewScanner(cr)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			lineChan <- scanner.Text()
		}
		close(lineChan)
	}()

	// Filter workers with WaitGroup
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range lineChan {
				kept := p.filterLine(line)
				resultChan <- filterResult{line: line, kept: kept}
			}
		}()
	}

	// Close resultChan when all workers done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Writer goroutine
	go func() {
		outFile, err := os.Create(p.outputFile)
		if err != nil {
			teaProgram.Send(pipeErrMsg{err: err})
			return
		}
		defer outFile.Close()

		writer := bufio.NewWriterSize(outFile, 256*1024)
		var written int64

		for result := range resultChan {
			teaProgram.Send(pipeLineMsg{kept: result.kept})
			if result.kept {
				n, _ := writer.WriteString(result.line + "\n")
				written += int64(n)
				if written >= 64*1024 {
					writer.Flush()
					teaProgram.Send(pipeBytesWrittenMsg{n: written})
					written = 0
				}
			}
		}
		if written > 0 {
			writer.Flush()
			teaProgram.Send(pipeBytesWrittenMsg{n: written})
		}
		writer.Flush()
		teaProgram.Send(pipeDoneMsg{})
	}()
}

type filterResult struct {
	line string
	kept bool
}

func (p *pipelineModel) filterLine(line string) bool {
	if p.minLen > 0 && len(line) < p.minLen {
		return false
	}
	if p.maxLen > 0 && len(line) > p.maxLen {
		return false
	}
	if p.asciiOnly && !asciiRegex.MatchString(line) {
		return false
	}
	return true
}

// countingReader wraps an io.Reader and reports bytes read
type countingReader struct {
	r          io.Reader
	teaProgram *tea.Program
	totalBytes int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	cr.totalBytes += int64(n)
	cr.teaProgram.Send(pipeBytesReadMsg{n: int64(n)})
	return n, err
}

// --- System Metrics ---

func getCPUTimePercent() float64 {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 15 {
		return 0
	}
	utime, _ := strconv.ParseFloat(fields[13], 64)
	stime, _ := strconv.ParseFloat(fields[14], 64)
	total := utime + stime

	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	uptimeFields := strings.Fields(string(uptimeData))
	if len(uptimeFields) == 0 {
		return 0
	}
	uptime, _ := strconv.ParseFloat(uptimeFields[0], 64)

	numCPU := float64(runtime.NumCPU())
	hz := 100.0
	return (total / hz / uptime / numCPU) * 100.0
}

func getRSSBytes() int64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

func getIOBytes() (read int64, write int64) {
	data, err := os.ReadFile("/proc/self/io")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "read_bytes:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				read, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		} else if strings.HasPrefix(line, "write_bytes:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				write, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}
	return read, write
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return string(out), err
}
