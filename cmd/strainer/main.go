package main

import (
	"fmt"
	"os"

	"github.com/Merovelous/strainer/internal/tui"
)

func main() {
	flags, isCLI := parseFlags()
	if isCLI {
		os.Exit(runCLI(flags))
	}
	if err := tui.Run(Version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
