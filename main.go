package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/shooooooooono/gotestui/collector"
	"github.com/shooooooooono/gotestui/view"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	flag.BoolVar(showVersion, "v", false, "Show version")
	importFile := flag.String("i", "", "Import test events from JSON file")
	flag.StringVar(importFile, "import", "", "Import test events from JSON file")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gotestui %s\n", version)
		return
	}

	eventChan := make(chan collector.TestEvent, 1000) // Buffered to prevent sender blocking
	doneChan := make(chan struct{})

	if *importFile != "" {
		go importFromFile(*importFile, eventChan, doneChan)
	} else {
		go readFromStdin(eventChan, doneChan)
	}

	view.CreateApplication(eventChan, doneChan)
}

func importFromFile(filename string, eventChan chan<- collector.TestEvent, doneChan chan struct{}) {
	defer close(doneChan)

	events, err := collector.ImportEvents(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error importing file: %v\n", err)
		return
	}
	for _, event := range events {
		eventChan <- event
	}
}

func readFromStdin(eventChan chan<- collector.TestEvent, doneChan chan struct{}) {
	if !isPipedInput() {
		fmt.Fprintln(os.Stderr, "Error: No piped input detected. Usage: go test -json ./... | gotestui")
		close(doneChan)
		return
	}
	collector.ReadLogStdin(bufio.NewScanner(os.Stdin), eventChan, doneChan)
}

// isPipedInput checks if stdin is receiving piped input
func isPipedInput() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}
