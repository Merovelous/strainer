package main

import (
	"context"
	"regexp"
	"time"
)

type viewState int

const (
	stateBrowser viewState = iota
	stateArchivePicker
	stateFilters
	stateOverwriteConfirm
	stateProcessing
	stateSummary
)

type pipeStatus int

const (
	pipeIdle pipeStatus = iota
	pipeRunning
	pipeDone
	pipeError
)

type appModel struct {
	state     viewState
	width     int
	height    int
	quitting  bool

	// Shared context
	inputFile           string // selected input file or archive
	selectedArchiveFile string // file inside archive (empty if not archive)
	isArchive           bool
	workingDir          string
	inputFileSize       int64 // size of the input file (for FPR estimates)

	// Filter config
	minLen      int
	maxLen      int
	asciiOnly   bool
	regexStr    string
	deduplicate bool
	bloomSize   int64

	// Output
	outputFile string

	// Sub-models
	browser       browserModel
	archivePicker archivePickerModel
	filters       filterModel
	processing    processingModel
	summary       summaryModel
}

type browserModel struct {
	entries      []entry
	cursor       int
	offset       int
	currentDir   string
	err          error
	ready        bool
	windowHeight int
}

type entry struct {
	name      string
	isDir     bool
	isArchive bool
	size      int64
	path      string
}

type archivePickerModel struct {
	entries      []archiveEntry
	cursor       int
	offset       int
	archivePath  string
	loading      bool
	err          error
	windowHeight int
}

type archiveEntry struct {
	name  string
	index int
}

type filterModel struct {
	options       []filterOption
	cursor        int
	inputing      bool
	inputBuf      string
	inputIdx      int
	fileName      string
	fileSize      int64
	validationErr string
}

type filterOption struct {
	name       string
	enabled    bool
	value      int    // numeric value (0 = unset)
	strValue   string // string value for regex pattern
	dynamic    bool   // true = numeric input field
	strDynamic bool   // true = string input field
	cycle      bool     // true = cycle through choices with ←/→
	choices    []string // options for cycle type
	choiceIdx  int      // current choice index
}

type metricsModel struct {
	startTime    time.Time
	cpuPct       float64
	rssBytes     int64
	prevCPUTicks float64
	prevCPUTime  time.Time
	// rolling EMA throughput — updated each metrics tick
	currentSpeed  float64
	currentLPS    float64
	prevBytesRead int64
	prevLinesRead int64
}

type pipelineModel struct {
	inputFile           string
	selectedArchiveFile string
	outputFile          string
	fileSize            int64
	minLen              int
	maxLen              int
	asciiOnly           bool
	isArchive           bool
	regex               *regexp.Regexp
	deduplicate         bool
	bloomSize           int64
	seen                map[string]struct{}

	// ctx/cancel allow the TUI to stop the goroutine. The goroutine checks
	// ctx.Done() between lines and cleans up the partial output file on cancel.
	ctx    context.Context
	cancel context.CancelFunc

	// done is closed by the goroutine when it finishes (success or error).
	// err and finishAt are written before close(done), so reading them after
	// observing the closed channel is safe without additional synchronization.
	done     chan struct{}
	status   pipeStatus
	startAt  time.Time
	finishAt time.Time
	err      error

	ready bool

	// Atomic counters — written by goroutines, read by TUI
	linesRead     int64
	linesKept     int64
	linesDropped  int64
	bytesRead     int64
	bytesWritten  int64
}

type summaryModel struct {
	inputFile    string
	outputFile   string
	linesRead    int64
	linesKept    int64
	linesDropped int64
	bytesRead    int64
	bytesWritten int64
	elapsed      time.Duration
	minLen      int
	maxLen      int
	asciiOnly   bool
	regexStr    string
	deduplicate bool
	ready       bool
	cancelled   bool
}

type processingModel struct {
	pipeline *pipelineModel
	metrics  metricsModel
}

// Message types
type (
	browserReadyMsg  struct{ entries []entry; err error }
	browserSelectMsg struct{ path string; isArchive bool }
	archiveReadyMsg  struct{ entries []archiveEntry; err error }
	archiveSelectMsg struct{ file string }
	pipeReadyMsg     struct{}
	pipeDoneMsg      struct{}
	pipeErrMsg       struct{ err error }
	metricsTickMsg   struct{}
)
