package collector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
