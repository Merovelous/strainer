package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync/atomic"
	"time"
)

// Status represents the pipeline execution state.
type Status int

const (
	Idle    Status = iota
	Running Status = iota
	Done    Status = iota
	Error   Status = iota
)

// maxArchiveOutput guards against archive bombs — stop writing after 10 GiB.
const maxArchiveOutput = 10 << 30

// Model is the pipeline engine. All fields are exported except seen.
type Model struct {
	InputFile           string
	SelectedArchiveFile string
	OutputFile          string
	FileSize            int64
	MinLen              int
	MaxLen              int
	ASCIIOnly           bool
	IsArchive           bool
	Regex               *regexp.Regexp
	Deduplicate         bool
	BloomSize           int64
	seen                map[string]struct{}

	// Ctx/Cancel allow the TUI to stop the goroutine. The goroutine checks
	// Ctx.Done() between lines and cleans up the partial output file on cancel.
	Ctx    context.Context
	Cancel context.CancelFunc

	// Done is closed by the goroutine when it finishes (success or error).
	// Err and FinishAt are written before close(Done), so reading them after
	// observing the closed channel is safe without additional synchronization.
	Done     chan struct{}
	Status   Status
	StartAt  time.Time
	FinishAt time.Time
	Err      error

	Ready bool

	// Atomic counters — written by goroutines, read by TUI
	LinesRead    int64
	LinesKept    int64
	LinesDropped int64
	BytesRead    int64
	BytesWritten int64
}

// Start launches the pipeline goroutine.
func (p *Model) Start() {
	p.StartAt = time.Now()
	p.Status = Running
	// seen map is only needed for the scanner-based fallback dedup path when bloom
	// is not in use (archives and plain files > 4 GB on Linux/macOS, or all files on Windows).
	if p.Deduplicate && (p.IsArchive || !canMmapDedup(p.FileSize)) && p.BloomSize == 0 {
		p.seen = make(map[string]struct{})
	}

	go func() {
		var runErr error
		defer func() {
			p.FinishAt = time.Now()
			if p.Ctx.Err() != nil {
				// User cancelled — delete the partial output file.
				os.Remove(p.OutputFile)
				p.Err = nil
			} else {
				p.Err = runErr
			}
			close(p.Done)
		}()

		// Allocate bloom filter for the fallback dedup path (archives / files > 4 GB).
		// mmap dedup always takes precedence for eligible plain files.
		var bloom *bloomFilter
		if p.Deduplicate && p.BloomSize > 0 && (p.IsArchive || !canMmapDedup(p.FileSize)) {
			if avail, ok := AvailableRAM(); ok && p.BloomSize > avail {
				runErr = fmt.Errorf("bloom filter needs %s but only %s RAM free",
					HumanSize(p.BloomSize), HumanSize(avail))
				return
			}
			bloom = newBloomFilter(p.BloomSize)
		}

		var reader io.Reader
		if p.IsArchive {
			args := []string{"x", p.InputFile, "-so", "-mmt=on"}
			if p.SelectedArchiveFile != "" {
				args = append(args, p.SelectedArchiveFile)
			}
			// CommandContext kills the 7z process when ctx is cancelled.
			cmd := exec.CommandContext(p.Ctx, SevenZipBin, args...)
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
			f, err := os.Open(p.InputFile)
			if err != nil {
				runErr = err
				return
			}
			defer f.Close()

			if p.Deduplicate && canMmapDedup(p.FileSize) {
				outFile, err := os.OpenFile(p.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
				if err != nil {
					runErr = err
					return
				}
				defer outFile.Close()
				w := bufio.NewWriterSize(outFile, 256*1024)
				runErr = mmapDedup(p, f, p.FileSize, w)
				if runErr == nil && p.Ctx.Err() == nil {
					w.Flush()
				}
				return
			}

			reader = f
		}

		cr := &atomicCounterReader{r: reader, bytesRead: &p.BytesRead}

		outFile, err := os.OpenFile(p.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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
			case <-p.Ctx.Done():
				return
			default:
			}
			if p.IsArchive && atomic.LoadInt64(&p.BytesWritten) > maxArchiveOutput {
				runErr = fmt.Errorf("output limit reached (%s) — possible archive bomb", HumanSize(maxArchiveOutput))
				return
			}
			atomic.AddInt64(&p.LinesRead, 1)
			line := scanner.Bytes()
			// Strip Windows CRLF — Scanner splits on \n but leaves \r on the token.
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if !p.filterLine(line) {
				atomic.AddInt64(&p.LinesDropped, 1)
				continue
			}
			if p.Deduplicate {
				var isDup bool
				if bloom != nil {
					isDup = bloom.seenAndAdd(line)
				} else {
					key := string(line)
					if _, exists := p.seen[key]; exists {
						isDup = true
					} else {
						p.seen[key] = struct{}{}
					}
				}
				if isDup {
					atomic.AddInt64(&p.LinesDropped, 1)
					continue
				}
			}
			atomic.AddInt64(&p.LinesKept, 1)
			writer.Write(line)
			writer.WriteByte('\n')
			atomic.AddInt64(&p.BytesWritten, int64(len(line))+1)
		}

		if err := scanner.Err(); err != nil {
			if p.Ctx.Err() == nil {
				runErr = err
			}
			return
		}

		writer.Flush()
	}()
}

// filterLine works on []byte directly — no string allocation
func (p *Model) filterLine(line []byte) bool {
	ll := len(line)
	if p.MinLen > 0 && ll < p.MinLen {
		return false
	}
	if p.MaxLen > 0 && ll > p.MaxLen {
		return false
	}
	if p.ASCIIOnly {
		for _, b := range line {
			if b < 0x20 || b > 0x7E {
				return false
			}
		}
	}
	if p.Regex != nil && !p.Regex.Match(line) {
		return false
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
