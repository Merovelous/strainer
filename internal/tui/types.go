package tui

import (
	"time"

	"github.com/Merovelous/strainer/internal/pipeline"
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

type appModel struct {
	state    viewState
	width    int
	height   int
	quitting bool
	version  string

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
	cycle      bool   // true = cycle through choices with ←/→
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

type processingModel struct {
	pipeline *pipeline.Model
	metrics  metricsModel
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
	regexStr     string
	deduplicate  bool
	ready        bool
	cancelled    bool
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
	filterConfirmMsg struct{}
)
