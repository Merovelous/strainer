package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Merovelous/strainer/internal/pipeline"
)

type cliFlags struct {
	input     string
	file      string
	output    string
	min       int
	max       int
	ascii     bool
	regexStr  string
	dedup     bool
	bloomSize string
	quiet     bool
}

// parseFlags parses CLI flags and returns them plus whether CLI mode is active.
// CLI mode is active when --input is provided.
func parseFlags() (cliFlags, bool) {
	var f cliFlags

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "strainer v%s — fast wordlist filter\n\n", Version)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  strainer --input <file> --output <file> [filters]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Required:")
		fmt.Fprintln(os.Stderr, "  --input  <file>    Wordlist file or archive (.7z .zip .tar.gz ...)")
		fmt.Fprintln(os.Stderr, "  --output <file>    Output file path")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Archive:")
		fmt.Fprintln(os.Stderr, "  --entry  <name>    File to extract from the archive (required when --input is an archive)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Filters (at least one recommended):")
		fmt.Fprintln(os.Stderr, "  --min    <n>       Keep lines with length >= n")
		fmt.Fprintln(os.Stderr, "  --max    <n>       Keep lines with length <= n")
		fmt.Fprintln(os.Stderr, "  --ascii            Keep ASCII-printable lines only (0x20-0x7E)")
		fmt.Fprintln(os.Stderr, "  --regex  <pat>     Keep lines matching regex pattern")
		fmt.Fprintln(os.Stderr, "  --dedup            Deduplicate lines")
		fmt.Fprintln(os.Stderr, "  --bloom-size <s>   Bloom filter RAM for --dedup on large files")
		fmt.Fprintln(os.Stderr, "                     Sizes: 256m 512m 1g 4g 8g or custom (e.g. 16g, 2048m)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Other:")
		fmt.Fprintln(os.Stderr, "  --quiet            Suppress progress output (errors still printed)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  strainer --input rockyou.txt --output out.txt --min 8 --max 12 --ascii")
		fmt.Fprintln(os.Stderr, "  strainer --input dump.7z --entry passwords.txt --output out.txt --dedup")
		fmt.Fprintln(os.Stderr, "  strainer --input huge.txt --output deduped.txt --dedup --bloom-size 4g")
	}

	flag.StringVar(&f.input, "input", "", "wordlist file or archive")
	flag.StringVar(&f.file, "entry", "", "file to extract from archive (required when --input is an archive)")
	flag.StringVar(&f.output, "output", "", "output file path")
	flag.IntVar(&f.min, "min", 0, "keep lines with length >= n (0 = no limit)")
	flag.IntVar(&f.max, "max", 0, "keep lines with length <= n (0 = no limit)")
	flag.BoolVar(&f.ascii, "ascii", false, "keep ASCII-printable lines only")
	flag.StringVar(&f.regexStr, "regex", "", "keep lines matching pattern")
	flag.BoolVar(&f.dedup, "dedup", false, "deduplicate lines")
	flag.StringVar(&f.bloomSize, "bloom-size", "", "bloom filter size for dedup (e.g. 1g, 4g, 2048m)")
	flag.BoolVar(&f.quiet, "quiet", false, "suppress progress output")
	flag.Parse()
	return f, f.input != ""
}

func runCLI(f cliFlags) int {
	if f.output == "" {
		fmt.Fprintln(os.Stderr, "error: --output is required")
		return 1
	}
	if _, err := os.Stat(f.input); err != nil {
		fmt.Fprintf(os.Stderr, "error: input not found: %s\n", f.input)
		return 1
	}

	isArchive := pipeline.IsArchiveFile(f.input)
	if isArchive && f.file == "" {
		fmt.Fprintln(os.Stderr, "error: --entry is required when input is an archive")
		return 1
	}

	if f.min > 0 && f.max > 0 && f.min > f.max {
		fmt.Fprintf(os.Stderr, "error: --min (%d) must be <= --max (%d)\n", f.min, f.max)
		return 1
	}

	noFilters := f.min == 0 && f.max == 0 && !f.ascii && f.regexStr == "" && !f.dedup
	if noFilters {
		fmt.Fprintln(os.Stderr, "warning: no filters specified — output will be identical to input (use --min, --max, --ascii, --regex, or --dedup)")
	}

	var re *regexp.Regexp
	if f.regexStr != "" {
		var err error
		re, err = regexp.Compile(f.regexStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid regex: %v\n", err)
			return 1
		}
	}

	var bloomBytes int64
	if f.bloomSize != "" {
		bloomBytes = pipeline.ParseBloomSize(f.bloomSize)
		if bloomBytes == 0 {
			fmt.Fprintf(os.Stderr, "error: invalid --bloom-size %q (use a number followed by m or g, e.g. 1g, 16g, 2048m)\n", f.bloomSize)
			return 1
		}
	}

	var fileSize int64
	if info, err := os.Stat(f.input); err == nil {
		fileSize = info.Size()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel on Ctrl+C / SIGTERM — pipeline cleans up partial output on cancel.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	p := &pipeline.Model{
		InputFile:           f.input,
		SelectedArchiveFile: f.file,
		OutputFile:          f.output,
		FileSize:            fileSize,
		MinLen:              f.min,
		MaxLen:              f.max,
		ASCIIOnly:           f.ascii,
		IsArchive:           isArchive,
		Regex:               re,
		Deduplicate:         f.dedup,
		BloomSize:           bloomBytes,
		Ctx:                 ctx,
		Cancel:              cancel,
		Done:                make(chan struct{}),
		Ready:               true,
	}

	p.Start()

	if !f.quiet {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
	loop:
		for {
			select {
			case <-ticker.C:
				lr := atomic.LoadInt64(&p.LinesRead)
				lk := atomic.LoadInt64(&p.LinesKept)
				br := atomic.LoadInt64(&p.BytesRead)
				fmt.Fprintf(os.Stderr, "\rprocessing...  %s read  %s kept  %s",
					pipeline.CommaFmt(lr), pipeline.CommaFmt(lk), pipeline.HumanSize(br))
			case <-p.Done:
				break loop
			}
		}
		fmt.Fprintln(os.Stderr)
	} else {
		<-p.Done
	}

	if p.Err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", p.Err)
		return 1
	}

	// Cancelled by signal
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "cancelled — partial output removed")
		return 1
	}

	elapsed := p.FinishAt.Sub(p.StartAt)
	var retPct float64
	if p.LinesRead > 0 {
		retPct = float64(p.LinesKept) / float64(p.LinesRead) * 100
	}
	fmt.Printf("%s kept / %s read (%.1f%%)  %s written  %s\n",
		pipeline.CommaFmt(p.LinesKept),
		pipeline.CommaFmt(p.LinesRead),
		retPct,
		pipeline.HumanSize(p.BytesWritten),
		pipeline.FormatDuration(elapsed))
	fmt.Printf("output: %s\n", f.output)

	return 0
}
