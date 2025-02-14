package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shooooooooono/gotestui/collector"
)

type TestCaseMap map[string][]collector.TestEvent

type TestEvent struct {
	Time    string           `json:"Time"`
	Action  collector.Action `json:"Action"`
	Package string           `json:"Package"`
	Test    string           `json:"Test,omitempty"`
	Elapsed float64          `json:"Elapsed,omitempty"`
	Output  string           `json:"Output,omitempty"`
}

// CreateApplication creates and starts the TUI application
func CreateApplication(eventChan <-chan collector.TestEvent, doneChan <-chan struct{}) {
	app := tview.NewApplication()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'q' {
			app.Stop()
			return nil
		}
		return event
	})

	// テスト結果のリストビューを作成
	list := tview.NewTreeView()
	root := tview.NewTreeNode(".")
	list.SetRoot(root).SetCurrentNode(root)

	testCases := make(TestCaseMap)
	nodeMap := make(map[string]*tview.TreeNode) // ノードの参照を保持

	// Outputを表示
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	usageView := tview.NewTextView().
		SetText("q: quit, hjkl/arrow: move").
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(false).
		SetMaxLines(1) // 1行だけ表示

	list.SetChangedFunc(func(node *tview.TreeNode) {
		viewLog(node, textView)
	})

	go func() {
		for {
			select {
			case te := <-eventChan:
				if te.IsRootEvent() {
					continue
				}
				testCases[te.Test] = append(testCases[te.Test], te)
				app.QueueUpdateDraw(func() {
					updateNode(root, nodeMap, te.Test, testCases[te.Test])
					viewLog(list.GetCurrentNode(), textView)
				})
				// map で集計したtestをrunAt 順に並び替える
			case <-doneChan:
				return
			}
		}
	}()

	// ログ・スタックトレースのテキストビューを作成

	// レイアウトを設定
	flex := tview.NewFlex().
		AddItem(list, 0, 1, true).
		AddItem(textView, 0, 2, false)

	footer := tview.NewFlex().AddItem(usageView, 0, 1, false)
	appFlex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(flex, 0, 1, true).AddItem(footer, 1, 0, false)

	// アプリケーションを設定して実行
	if err := app.SetRoot(appFlex, true).Run(); err != nil {
		panic(err)
	}
}

func updateNode(root *tview.TreeNode, nodeMap map[string]*tview.TreeNode, testName string, events []collector.TestEvent) {
	parts := strings.Split(testName, "/")
	parent := root

	for i, part := range parts {
		path := strings.Join(parts[:i+1], "/")
		node, exists := nodeMap[path]
		if !exists {
			node = tview.NewTreeNode(part)
			nodeMap[path] = node
			parent.AddChild(node)
		}
		parent = node
	}

	statusIcon := "-"
	color := tcell.ColorDefault
	for _, te := range events {
		switch te.Action {
		case collector.ActionPass:
			statusIcon = ""
			color = tcell.ColorGreen
		case collector.ActionFail:
			statusIcon = ""
			color = tcell.ColorRed
		case collector.ActionRun:
			statusIcon = ""
			color = tcell.ColorGray
		case collector.ActionSkip:
			statusIcon = "󰼦"
			color = tcell.ColorDefault
		case collector.ActionStart:
			color = tcell.ColorYellow
		}
	}

	parent.SetReference(events)
	text := fmt.Sprintf("%s - %s", statusIcon, testName)
	parent.SetText(text).SetColor(color)
}

func getTestEvent(node *tview.TreeNode) []collector.TestEvent {
	ref := node.GetReference()
	if ref == nil {
		return nil
	}
	return ref.([]collector.TestEvent)
}

func viewLog(node *tview.TreeNode, textView *tview.TextView) {
	textView.SetText("")
	if node.GetText() == "." {
		textView.SetText("select testcase")
		return
	}
	var text string
	var runAt time.Time
	events := getTestEvent(node)
	for _, event := range events {
		text += event.Output

		if event.Action == collector.ActionRun {
			runAt = event.Time
		}
	}
	textView.SetText(text + " -> " + runAt.Format(time.DateTime))
}
