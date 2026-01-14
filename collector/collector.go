package collector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type Action string

const (
	ActionRun    Action = "run"
	ActionPass   Action = "pass"
	ActionFail   Action = "fail"
	ActionSkip   Action = "skip"
	ActionOutput Action = "output"
	ActionStart  Action = "start"
)

type TestEvent struct {
	Time    time.Time `json:"Time"`
	Action  Action    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test,omitempty"`
	Elapsed float64   `json:"Elapsed,omitempty"`
	Output  string    `json:"Output,omitempty"`
}

func (te *TestEvent) IsRootEvent() bool {
	return te.Test == ""
}

type Results struct {
	Passed  int
	Failed  int
	Skipped int
}

func UnmarshalTestEvent(b []byte) (TestEvent, error) {
	var te TestEvent
	err := json.Unmarshal(b, &te)
	if err != nil {
		return TestEvent{}, err
	}
	return te, nil
}

func ReadLogStdin(scanner *bufio.Scanner, eventChan chan<- TestEvent, doneChan chan<- struct{}) {
	defer close(doneChan)

	for scanner.Scan() {
		line := scanner.Bytes()
		te, err := UnmarshalTestEvent(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to parse JSON: %v (line: %s)\n", err, string(line))
			continue
		}
		eventChan <- te
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

// ExportEvents exports test events to a JSON file (one event per line, same as go test -json)
func ExportEvents(filename string, events []TestEvent) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return fmt.Errorf("failed to encode event: %w", err)
		}
	}
	return nil
}

// ImportEvents imports test events from a JSON file (one event per line, same as go test -json)
func ImportEvents(filename string) ([]TestEvent, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var events []TestEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		te, err := UnmarshalTestEvent(scanner.Bytes())
		if err != nil {
			continue // Skip invalid lines
		}
		events = append(events, te)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return events, nil
}

// RunPackage executes all tests in a package and sends events to the channel
func RunPackage(pkg string, eventChan chan<- TestEvent) error {
	return runGoTest(eventChan, "go", "test", "-json", pkg)
}

// RunTest executes a specific test and sends events to the channel
func RunTest(pkg string, testName string, eventChan chan<- TestEvent) error {
	return runGoTest(eventChan, "go", "test", "-json", "-run", "^"+testName+"$", pkg)
}

// runGoTest executes a go test command and streams events to the channel
func runGoTest(eventChan chan<- TestEvent, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start test: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		te, err := UnmarshalTestEvent(scanner.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to parse JSON: %v\n", err)
			continue
		}
		eventChan <- te
	}

	if err := cmd.Wait(); err != nil {
		// Test failures result in ExitError, which is expected
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("test command failed: %w", err)
		}
	}

	return nil
}
