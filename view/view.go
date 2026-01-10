package view

import (
	"fmt"
	"strings"

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

	list := tview.NewTreeView()
	root := tview.NewTreeNode(".")
	root.SetExpanded(true) // ルートをデフォルトで展開
	list.SetRoot(root).SetCurrentNode(root)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		currentNode := list.GetCurrentNode()
		if currentNode == nil {
			return event
		}

		// Toggle expand/collapse with Enter or Space
		if event.Key() == tcell.KeyEnter || (event.Key() == tcell.KeyRune && event.Rune() == ' ') {
			currentNode.SetExpanded(!currentNode.IsExpanded())
			return nil
		}
		return event
	})

	testCases := make(TestCaseMap)
	nodeMap := make(map[string]*tview.TreeNode)

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	usageView := tview.NewTextView().
		SetText("q: quit, hjkl/arrow: move, Enter/Space: expand/collapse").
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
			node.SetExpanded(true) // デフォルトで展開
			nodeMap[path] = node
			parent.AddChild(node)
		}
		parent = node
	}

	statusIcon := "⧗"
	color := tcell.ColorDefault
	var elapsed float64

	for _, te := range events {
		if te.Elapsed > 0 {
			elapsed = te.Elapsed
		}
		switch te.Action {
		case collector.ActionPass:
			statusIcon = "✓"
			color = tcell.ColorGreen
		case collector.ActionFail:
			statusIcon = "✗"
			color = tcell.ColorRed
		case collector.ActionRun:
			statusIcon = "▶"
			color = tcell.ColorYellow
		case collector.ActionSkip:
			statusIcon = "⏭"
			color = tcell.ColorDarkCyan
		case collector.ActionStart:
			statusIcon = "⧗"
			color = tcell.ColorGray
		}
	}

	parent.SetReference(events)

	// Format with icon, name, and elapsed time
	testDisplayName := parts[len(parts)-1]
	var text string
	if elapsed > 0 {
		text = fmt.Sprintf("%s %s [%.3fs]", statusIcon, testDisplayName, elapsed)
	} else {
		text = fmt.Sprintf("%s %s", statusIcon, testDisplayName)
	}

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
	events := getTestEvent(node)
	for _, event := range events {
		text += event.Output
	}
	textView.SetText(text)
}
