package main

import (
	"bufio"
	"os"

	"github.com/shooooooooono/gotestui/collector"
	"github.com/shooooooooono/gotestui/view"
)

func main() {
	eventChan := make(chan collector.TestEvent)
	doneChan := make(chan struct{})

	// 標準入力からのデータを読み取る
	stdinScanner := bufio.NewScanner(os.Stdin)
	go collector.ReadLogStdout(stdinScanner, eventChan, doneChan)

	// TUIを作成
	view.CreateApplication(eventChan, doneChan)
}
