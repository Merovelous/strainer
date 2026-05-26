package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pipeline with only the filter fields set — enough for filterLine tests.
func newTestPipeline(minLen, maxLen int, asciiOnly bool, regex *regexp.Regexp, deduplicate bool) *pipelineModel {
	return &pipelineModel{
		minLen:      minLen,
		maxLen:      maxLen,
		asciiOnly:   asciiOnly,
		regex:       regex,
		deduplicate: deduplicate,
	}
}

func TestFilterLine_MinLen(t *testing.T) {
	p := newTestPipeline(5, 0, false, nil, false)
	tests := []struct {
		line string
		want bool
	}{
		{"abcde", true},   // exactly at min
		{"abcdef", true},  // above min
		{"abcd", false},   // one below min
		{"", false},       // empty
		{"a", false},      // way below
	}
	for _, tt := range tests {
		if got := p.filterLine([]byte(tt.line)); got != tt.want {
			t.Errorf("filterLine(%q) minLen=5 = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestFilterLine_MaxLen(t *testing.T) {
	p := newTestPipeline(0, 8, false, nil, false)
	tests := []struct {
		line string
		want bool
	}{
		{"abcdefgh", true},   // exactly at max
		{"abcdefg", true},    // below max
		{"abcdefghi", false}, // one above max
		{"", true},           // empty passes (no min)
	}
	for _, tt := range tests {
		if got := p.filterLine([]byte(tt.line)); got != tt.want {
			t.Errorf("filterLine(%q) maxLen=8 = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestFilterLine_MinMaxCombined(t *testing.T) {
	p := newTestPipeline(4, 8, false, nil, false)
	tests := []struct {
		line string
		want bool
	}{
		{"abcd", true},       // at min boundary
		{"abcdefgh", true},   // at max boundary
		{"abc", false},       // below min
		{"abcdefghi", false}, // above max
		{"abcde", true},      // in range
	}
	for _, tt := range tests {
		if got := p.filterLine([]byte(tt.line)); got != tt.want {
			t.Errorf("filterLine(%q) min=4 max=8 = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestFilterLine_ASCIIOnly(t *testing.T) {
	p := newTestPipeline(0, 0, true, nil, false)
	tests := []struct {
		line []byte
		want bool
		desc string
	}{
		{[]byte("hello123"), true, "printable ASCII"},
		{[]byte("Hello World!"), true, "ASCII with space and !"},
		{[]byte{0x1F, 0x61}, false, "control char 0x1F"},
		{[]byte{0x7F, 0x61}, false, "DEL 0x7F"},
		{[]byte{0x80, 0x61}, false, "non-ASCII 0x80"},
		{[]byte{0xFF, 0x61}, false, "non-ASCII 0xFF"},
		{[]byte("!@#$%^&*()"), true, "special printable chars"},
		{[]byte{0x20}, true, "space 0x20 (min printable)"},
		{[]byte{0x7E}, true, "tilde 0x7E (max printable)"},
		{[]byte{0x00}, false, "null byte"},
	}
	for _, tt := range tests {
		if got := p.filterLine(tt.line); got != tt.want {
			t.Errorf("filterLine ASCII %s = %v, want %v", tt.desc, got, tt.want)
		}
	}
}

func TestFilterLine_Regex(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	p := newTestPipeline(0, 0, false, re, false)
	tests := []struct {
		line string
		want bool
	}{
		{"hello", true},
		{"world", true},
		{"Hello", false},  // uppercase
		{"hello1", false}, // digit
		{"", false},       // empty doesn't match [a-z]+
	}
	for _, tt := range tests {
		if got := p.filterLine([]byte(tt.line)); got != tt.want {
			t.Errorf("filterLine(%q) regex=[a-z]+ = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestFilterLine_AllFilters(t *testing.T) {
	re := regexp.MustCompile(`^[a-z0-9]+$`)
	p := newTestPipeline(4, 8, true, re, false)
	tests := []struct {
		line string
		want bool
		desc string
	}{
		{"abcd", true, "min boundary, lowercase"},
		{"abc", false, "below min"},
		{"abcdefghi", false, "above max"},
		{"abcDEF", false, "fails regex (uppercase)"},
		{"abc123", true, "alphanumeric in range"},
	}
	for _, tt := range tests {
		if got := p.filterLine([]byte(tt.line)); got != tt.want {
			t.Errorf("filterLine %s = %v, want %v", tt.desc, got, tt.want)
		}
	}
}

// Integration test: write a temp wordlist, run the pipeline, check output.
func TestPipelineIntegration(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.txt")
	output := filepath.Join(t.TempDir(), "output.txt")

	lines := []string{
		"hello",      // 5 chars, ASCII — kept
		"hi",         // 2 chars — dropped by min=4
		"worldworld", // 10 chars — dropped by max=6
		"héllo",      // non-ASCII — dropped by ascii
		"strainer",   // 8 chars — dropped by max=6
		"pass1",      // 5 chars, ASCII — kept
		"hello",      // duplicate — dropped by dedup
		"PASS2",      // 5 chars, ASCII — kept
	}
	if err := os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &pipelineModel{
		inputFile:   input,
		outputFile:  output,
		minLen:      4,
		maxLen:      6,
		asciiOnly:   true,
		deduplicate: true,
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
		ready:       true,
	}

	p.start()
	<-p.done

	if p.err != nil {
		t.Fatalf("pipeline error: %v", p.err)
	}

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	want := "hello\npass1\nPASS2\n"
	if string(got) != want {
		t.Errorf("output =\n%q\nwant\n%q", string(got), want)
	}

	if p.linesRead != int64(len(lines)) {
		t.Errorf("linesRead = %d, want %d", p.linesRead, len(lines))
	}
	if p.linesKept != 3 {
		t.Errorf("linesKept = %d, want 3", p.linesKept)
	}
}

func TestPipelineCRLF(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.txt")
	output := filepath.Join(t.TempDir(), "output.txt")

	// Write CRLF line endings
	content := "hello\r\nworld\r\nhi\r\n"
	if err := os.WriteFile(input, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &pipelineModel{
		inputFile:  input,
		outputFile: output,
		minLen:     4,
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
		ready:      true,
	}

	p.start()
	<-p.done

	if p.err != nil {
		t.Fatalf("pipeline error: %v", p.err)
	}

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	// CRLF stripped — output should have Unix newlines, no \r
	if strings.Contains(string(got), "\r") {
		t.Errorf("output contains \\r — CRLF not stripped: %q", string(got))
	}
	want := "hello\nworld\n"
	if string(got) != want {
		t.Errorf("output = %q, want %q", string(got), want)
	}
}

func TestBuildOutputName(t *testing.T) {
	tests := []struct {
		input string
		setup func(*filterModel)
		want  string
	}{
		{
			"/path/to/rockyou.txt",
			func(f *filterModel) {
				f.options[0].enabled = true
				f.options[0].value = 8
				f.options[1].enabled = true
				f.options[1].value = 12
			},
			"rockyou_8to12.txt",
		},
		{
			"/path/to/weakpass.txt",
			func(f *filterModel) {
				f.options[0].enabled = true
				f.options[0].value = 6
				f.options[2].enabled = true // ascii
			},
			"weakpass_min6_ascii.txt",
		},
		{
			"/path/to/dump.tar.gz",
			func(f *filterModel) {
				f.options[1].enabled = true
				f.options[1].value = 16
			},
			"dump_max16.txt",
		},
		{
			"/path/to/words.txt",
			func(f *filterModel) {},
			"words_filtered.txt",
		},
		{
			"/path/to/archive.tar.bz2",
			func(f *filterModel) {
				f.options[4].enabled = true // dedup
			},
			"archive_dedup.txt",
		},
	}
	for _, tt := range tests {
		f := newFilterModel(tt.input, 0)
		tt.setup(&f)
		got := f.buildOutputName(tt.input)
		if got != tt.want {
			t.Errorf("buildOutputName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct{ n int64; want string }{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024), "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, tt := range tests {
		if got := humanSize(tt.n); got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCommaFmt(t *testing.T) {
	tests := []struct{ n int64; want string }{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1000000, "1,000,000"},
		{42976092, "42,976,092"},
	}
	for _, tt := range tests {
		if got := commaFmt(tt.n); got != tt.want {
			t.Errorf("commaFmt(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// TestMmapDedup verifies that the mmap-based dedup path produces byte-identical
// output to the expected unique-line set. Skipped on platforms where mmap dedup
// is unavailable (Windows).
func TestMmapDedup(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.txt")
	output := filepath.Join(t.TempDir(), "output.txt")

	content := "apple\nbanana\napple\ncherry\nbanana\ndate\napple\n"
	if err := os.WriteFile(input, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(input)
	if err != nil {
		t.Fatal(err)
	}
	if !canMmapDedup(info.Size()) {
		t.Skip("mmap dedup not supported on this platform")
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &pipelineModel{
		inputFile:   input,
		outputFile:  output,
		fileSize:    info.Size(),
		deduplicate: true,
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
		ready:       true,
	}

	p.start()
	<-p.done

	if p.err != nil {
		t.Fatalf("pipeline error: %v", p.err)
	}

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	want := "apple\nbanana\ncherry\ndate\n"
	if string(got) != want {
		t.Errorf("mmap dedup output =\n%q\nwant\n%q", string(got), want)
	}
	if p.linesRead != 7 {
		t.Errorf("linesRead = %d, want 7", p.linesRead)
	}
	if p.linesKept != 4 {
		t.Errorf("linesKept = %d, want 4", p.linesKept)
	}
	if p.linesDropped != 3 {
		t.Errorf("linesDropped = %d, want 3", p.linesDropped)
	}
}

// TestBloomDedup verifies that the bloom filter dedup path produces correct output
// (no false negatives; occasional false positives are acceptable but don't occur
// for a tiny test set with a generously sized filter).
func TestBloomDedup(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.txt")
	output := filepath.Join(t.TempDir(), "output.txt")

	content := "apple\nbanana\napple\ncherry\nbanana\ndate\napple\n"
	if err := os.WriteFile(input, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &pipelineModel{
		inputFile:  input,
		outputFile: output,
		// Fake a fileSize beyond the mmap limit so canMmapDedup returns false,
		// forcing the scanner + bloom path without touching the archive branch.
		fileSize:    mmapMaxFileSize + 1,
		deduplicate: true,
		bloomSize:   1 << 20, // 1 MB — tiny filter, zero FPR for 7 lines
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
		ready:       true,
	}

	p.start()
	<-p.done

	if p.err != nil {
		t.Fatalf("pipeline error: %v", p.err)
	}

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	want := "apple\nbanana\ncherry\ndate\n"
	if string(got) != want {
		t.Errorf("bloom dedup output =\n%q\nwant\n%q", string(got), want)
	}
	if p.linesRead != 7 {
		t.Errorf("linesRead = %d, want 7", p.linesRead)
	}
	if p.linesKept != 4 {
		t.Errorf("linesKept = %d, want 4", p.linesKept)
	}
	if p.linesDropped != 3 {
		t.Errorf("linesDropped = %d, want 3", p.linesDropped)
	}
}

// Ensure humanSize is accessible — it's defined in summary.go / processing.go.
// This compile-time check confirms the test file is in the right package.
var _ = humanSize
var _ = bufio.NewScanner
