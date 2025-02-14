package collector

import (
	"bufio"
	"encoding/json"
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

func ReadLogStdout(scanner *bufio.Scanner, eventChan chan<- TestEvent, doneChan chan<- struct{}) {
	defer close(doneChan)

	for scanner.Scan() {
		line := scanner.Bytes()
		te, err := UnmarshalTestEvent(line)
		if err != nil {
			continue
		}
		eventChan <- te
	}
}
