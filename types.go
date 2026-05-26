package main

import "time"

type viewState int

const (
	stateBrowser viewState = iota
	stateArchivePicker
	stateFilters
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
	inputFile          string // selected input file or archive
	selectedArchiveFile string // file inside archive (empty if not archive)
	isArchive          bool
	workingDir         string

	// Filter config
	minLen    int
	maxLen    int
	asciiOnly bool

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
	entries     []archiveEntry
	cursor      int
	offset      int
	archivePath string
	loading     bool
	err         error
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
	validationErr string
}

type filterOption struct {
	name    string
	enabled bool
	value   int    // 0 = toggle-only
	dynamic bool   // true = has input field
}

type metricsModel struct {
	startTime    time.Time
	cpuPct       float64
	rssBytes     int64
	ioReadBytes  int64
	ioWriteBytes int64
	prevCPUTicks float64
	prevCPUTime  time.Time
}

type pipelineModel struct {
	inputFile  string
	outputFile string
	fileSize   int64 // source file size for progress
	minLen     int
	maxLen     int
	asciiOnly  bool
	isArchive  bool

	status   pipeStatus
	startAt  time.Time
	finishAt time.Time
	err      error

	ready bool // set true to trigger Start()

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
	minLen       int
	maxLen       int
	asciiOnly    bool
	ready        bool
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
