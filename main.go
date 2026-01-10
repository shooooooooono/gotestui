package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/shooooooooono/gotestui/collector"
	"github.com/shooooooooono/gotestui/view"
)

func main() {
	eventChan := make(chan collector.TestEvent)
	doneChan := make(chan struct{})

	// 標準入力からのデータを読み取る
	stdinScanner := bufio.NewScanner(os.Stdin)
	go func() {
		if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
			collector.ReadLogStdin(stdinScanner, eventChan, doneChan)
		} else {
			fmt.Fprintln(os.Stderr, "Error: No piped input detected. Usage: go test -json ./... | gotestui")
			close(doneChan)
		}
	}()

	// TUIを作成
	view.CreateApplication(eventChan, doneChan)
}
