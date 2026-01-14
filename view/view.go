package view

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shooooooooono/gotestui/collector"
)

type TestCaseMap map[string][]collector.TestEvent

// HistoryState represents the state of a history
type HistoryState int

const (
	StateIdle      HistoryState = iota // Initial/pipe input completed
	StateRunning                       // Running
	StateCompleted                     // Success
	StateFailed                        // Failed
)

// History represents a single test run session
type History struct {
	Name      string
	TestCases TestCaseMap
	NodeMap   map[string]*tview.TreeNode
	Root      *tview.TreeNode
	State     HistoryState
	mu        sync.Mutex
}

// NewHistory creates a new history with the given name
func NewHistory(name string) *History {
	root := tview.NewTreeNode(".")
	root.SetExpanded(true)
	return &History{
		Name:      name,
		TestCases: make(TestCaseMap),
		NodeMap:   make(map[string]*tview.TreeNode),
		Root:      root,
	}
}

// HistoryManager manages multiple test histories
type HistoryManager struct {
	Histories    []*History
	CurrentIndex int
}

// NewHistoryManager creates a new history manager
func NewHistoryManager() *HistoryManager {
	return &HistoryManager{
		Histories:    []*History{},
		CurrentIndex: 0,
	}
}

// AddHistory adds a new history and returns it
func (hm *HistoryManager) AddHistory(name string) *History {
	h := NewHistory(name)
	hm.Histories = append(hm.Histories, h)
	hm.CurrentIndex = len(hm.Histories) - 1
	return h
}

// Current returns the current history
func (hm *HistoryManager) Current() *History {
	if len(hm.Histories) == 0 {
		return nil
	}
	return hm.Histories[hm.CurrentIndex]
}

// Next switches to the next history
func (hm *HistoryManager) Next() bool {
	if hm.CurrentIndex < len(hm.Histories)-1 {
		hm.CurrentIndex++
		return true
	}
	return false
}

// Prev switches to the previous history
func (hm *HistoryManager) Prev() bool {
	if hm.CurrentIndex > 0 {
		hm.CurrentIndex--
		return true
	}
	return false
}

// CreateApplication creates and starts the TUI application
func CreateApplication(eventChan <-chan collector.TestEvent, doneChan <-chan struct{}) {
	app := tview.NewApplication()

	historyMgr := NewHistoryManager()
	initialHistory := historyMgr.AddHistory("Initial")
	initialHistory.State = StateRunning

	// History list
	historyList := tview.NewList()
	historyList.SetBorder(true).SetTitle("History").SetBorderColor(tcell.ColorGray)
	historyList.ShowSecondaryText(false)
	historyList.AddItem("▶ Initial", "", 0, nil)

	// Test tree
	treeView := tview.NewTreeView()
	treeView.SetBorder(true).SetTitle("Tests").SetBorderColor(tcell.ColorWhite) // Initial focus
	treeView.SetGraphics(false) // Use indentation instead of tree lines
	treeView.SetRoot(initialHistory.Root).SetCurrentNode(initialHistory.Root)

	// Log view
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetScrollable(true)
	textView.SetBorder(true).SetTitle("Log").SetBorderColor(tcell.ColorGray)

	// Search state
	searchQuery := ""
	searchMatches := []int{} // Line numbers with matches
	searchIndex := 0

	// Search input field
	searchInput := tview.NewInputField().
		SetLabel("/").
		SetFieldWidth(0)
	searchInput.SetBorder(false)

	// Log panel with search input
	logPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true)
	searchMode := false

	showSearchInput := func() {
		if !searchMode {
			searchMode = true
			logPanel.AddItem(searchInput, 1, 0, false)
		}
	}

	hideSearchInput := func() {
		if searchMode {
			searchMode = false
			logPanel.RemoveItem(searchInput)
		}
	}

	// Update border colors based on focus
	updateFocus := func(p tview.Primitive) {
		historyList.SetBorderColor(tcell.ColorGray)
		treeView.SetBorderColor(tcell.ColorGray)
		textView.SetBorderColor(tcell.ColorGray)
		switch p {
		case historyList:
			historyList.SetBorderColor(tcell.ColorWhite)
		case treeView:
			treeView.SetBorderColor(tcell.ColorWhite)
		case textView:
			textView.SetBorderColor(tcell.ColorWhite)
		}
	}

	// Find all matching lines
	findMatches := func(query string) {
		searchMatches = []int{}
		if query == "" {
			return
		}
		content := textView.GetText(true)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				searchMatches = append(searchMatches, i)
			}
		}
		searchIndex = 0
	}

	// Jump to match and re-render with current match highlighted
	jumpToMatch := func(index int) {
		if len(searchMatches) == 0 {
			return
		}
		if index < 0 {
			index = len(searchMatches) - 1
		} else if index >= len(searchMatches) {
			index = 0
		}
		searchIndex = index
		viewLog(treeView.GetCurrentNode(), textView, searchQuery, searchIndex)
		textView.ScrollTo(searchMatches[searchIndex], 0)
		textView.SetTitle(fmt.Sprintf("Log [%d/%d]", searchIndex+1, len(searchMatches)))
	}

	// Search input handlers
	searchInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			searchQuery = searchInput.GetText()
			findMatches(searchQuery)
			if len(searchMatches) > 0 {
				jumpToMatch(0)
			} else {
				viewLog(treeView.GetCurrentNode(), textView, searchQuery, -1)
				textView.SetTitle("Log [no match]")
			}
		} else {
			searchQuery = ""
			viewLog(treeView.GetCurrentNode(), textView, searchQuery, -1)
			textView.SetTitle("Log")
		}
		hideSearchInput()
		searchInput.SetText("")
		app.SetFocus(textView)
	})

	// Log view input handling
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'j':
				row, col := textView.GetScrollOffset()
				textView.ScrollTo(row+1, col)
				return nil
			case 'k':
				row, col := textView.GetScrollOffset()
				if row > 0 {
					textView.ScrollTo(row-1, col)
				}
				return nil
			case 'g':
				textView.ScrollToBeginning()
				return nil
			case 'G':
				textView.ScrollToEnd()
				return nil
			case '/':
				showSearchInput()
				app.SetFocus(searchInput)
				return nil
			case 'n':
				if len(searchMatches) > 0 {
					jumpToMatch(searchIndex + 1)
				}
				return nil
			case 'N':
				if len(searchMatches) > 0 {
					jumpToMatch(searchIndex - 1)
				}
				return nil
			}
		}
		// Enter to go back to tree view
		if event.Key() == tcell.KeyEnter {
			app.SetFocus(treeView)
			updateFocus(treeView)
			return nil
		}
		// Escape to clear search
		if event.Key() == tcell.KeyEsc {
			searchQuery = ""
			searchMatches = []int{}
			viewLog(treeView.GetCurrentNode(), textView, searchQuery, -1)
			textView.SetTitle("Log")
			return nil
		}
		return event
	})

	// Usage view
	usageView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(false).
		SetMaxLines(1)
	usageView.SetText("q: quit, Tab: focus, Space: expand, Enter: log, r: rerun, e: export, /: search, n/N: next/prev")

	// Flag to prevent recursive updates
	updatingHistoryList := false
	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	var spinnerFrame atomic.Int32

	// Update history list display
	updateHistoryList := func() {
		if updatingHistoryList {
			return
		}
		updatingHistoryList = true
		defer func() { updatingHistoryList = false }()

		historyList.Clear()
		for i, h := range historyMgr.Histories {
			prefix := "  "
			if i == historyMgr.CurrentIndex {
				prefix = "▶ "
			}

			suffix := historyStateSuffix(h.State, spinnerFrames[spinnerFrame.Load()])

			historyList.AddItem(fmt.Sprintf("%s%s%s", prefix, h.Name, suffix), "", 0, nil)
		}
		historyList.SetCurrentItem(historyMgr.CurrentIndex)
	}

	// Animation ticker for running histories and tests
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			spinnerFrame.Store((spinnerFrame.Load() + 1) % int32(len(spinnerFrames)))

			hasRunning := false
			for _, h := range historyMgr.Histories {
				if h.State == StateRunning {
					hasRunning = true
					break
				}
			}

			if hasRunning {
				app.QueueUpdateDraw(func() {
					updateHistoryList()
					// Update running test nodes in current history
					currentHistory := historyMgr.Current()
					if currentHistory != nil && currentHistory.State == StateRunning {
						currentHistory.mu.Lock()
						for testName, events := range currentHistory.TestCases {
							if len(events) > 0 && isTestRunning(events) {
								updateNode(currentHistory.Root, currentHistory.NodeMap, testName, events, spinnerFrames[spinnerFrame.Load()])
							}
						}
						currentHistory.mu.Unlock()
					}
				})
			}
		}
	}()

	// Switch to a history and update the view
	switchHistory := func(index int) {
		if index < 0 || index >= len(historyMgr.Histories) {
			return
		}
		historyMgr.CurrentIndex = index
		h := historyMgr.Current()
		if h != nil {
			// Reset search state when switching histories
			searchQuery = ""
			searchMatches = []int{}
			searchIndex = 0
			textView.SetTitle("Log")
			treeView.SetRoot(h.Root).SetCurrentNode(h.Root)
			updateHistoryList()
			viewLog(treeView.GetCurrentNode(), textView, searchQuery, -1)
		}
	}

	// History list selection handler - use SetSelectedFunc (only fires on Enter)
	historyList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		switchHistory(index)
		app.SetFocus(treeView)
		updateFocus(treeView)
	})

	// Also handle cursor movement to preview history
	historyList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if updatingHistoryList {
			return
		}
		switchHistory(index)
	})

	// Add vim-style navigation to history list
	historyList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'j':
				current := historyList.GetCurrentItem()
				if current < historyList.GetItemCount()-1 {
					historyList.SetCurrentItem(current + 1)
				}
				return nil
			case 'k':
				current := historyList.GetCurrentItem()
				if current > 0 {
					historyList.SetCurrentItem(current - 1)
				}
				return nil
			}
		}
		return event
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'q' {
			app.Stop()
			return nil
		}
		// Tab to switch focus between history list, tree view, and log view
		if event.Key() == tcell.KeyTab {
			var next tview.Primitive
			switch app.GetFocus() {
			case historyList:
				next = treeView
			case treeView:
				next = textView
			default:
				next = historyList
			}
			app.SetFocus(next)
			updateFocus(next)
			return nil
		}
		// Export current history with 'e'
		if event.Key() == tcell.KeyRune && event.Rune() == 'e' {
			h := historyMgr.Current()
			if h != nil {
				// Collect all events from current history
				h.mu.Lock()
				var allEvents []collector.TestEvent
				for _, events := range h.TestCases {
					allEvents = append(allEvents, events...)
				}
				h.mu.Unlock()
				if len(allEvents) > 0 {
					filename := fmt.Sprintf("gotestui-export-%s.json", time.Now().Format("20060102-150405"))
					if err := collector.ExportEvents(filename, allEvents); err != nil {
						textView.SetText(fmt.Sprintf("Export failed: %v", err))
					} else {
						textView.SetText(fmt.Sprintf("Exported to %s (%d events)", filename, len(allEvents)))
					}
				}
			}
			return nil
		}
		// Escape to go back to tree view
		if event.Key() == tcell.KeyEsc {
			app.SetFocus(treeView)
			updateFocus(treeView)
			return nil
		}
		return event
	})

	treeView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		currentNode := treeView.GetCurrentNode()
		if currentNode == nil {
			return event
		}

		// Toggle expand/collapse with Space
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			currentNode.SetExpanded(!currentNode.IsExpanded())
			updateNodeExpandIcon(currentNode)
			return nil
		}

		// Enter to focus log view
		if event.Key() == tcell.KeyEnter {
			app.SetFocus(textView)
			updateFocus(textView)
			return nil
		}

		// Rerun test with 'r' - creates a new history
		if event.Key() == tcell.KeyRune && event.Rune() == 'r' {
			rerunTarget := parseRerunTarget(currentNode.GetReference())
			if rerunTarget == nil {
				return nil
			}

			rerunHistory := historyMgr.AddHistory(rerunTarget.historyName)
			rerunHistory.State = StateRunning
			treeView.SetRoot(rerunHistory.Root).SetCurrentNode(rerunHistory.Root)
			updateHistoryList()

			rerunChan := make(chan collector.TestEvent, 100)

			// Process events for this rerun
			go func() {
				for te := range rerunChan {
					if te.IsRootEvent() {
						continue
					}
					rerunHistory.mu.Lock()
					rerunHistory.TestCases[te.Test] = append(rerunHistory.TestCases[te.Test], te)
					testName := te.Test
					eventsCopy := make([]collector.TestEvent, len(rerunHistory.TestCases[te.Test]))
					copy(eventsCopy, rerunHistory.TestCases[te.Test])
					rerunHistory.mu.Unlock()
					app.QueueUpdateDraw(func() {
						updateNode(rerunHistory.Root, rerunHistory.NodeMap, testName, eventsCopy, spinnerFrames[spinnerFrame.Load()])
						if historyMgr.Current() == rerunHistory {
							viewLog(treeView.GetCurrentNode(), textView, searchQuery, -1)
						}
					})
				}
			}()

			// Run the test
			go func() {
				defer close(rerunChan)
				defer func() {
					rerunHistory.mu.Lock()
					rerunHistory.State = stateFromTestResult(rerunHistory.TestCases)
					rerunHistory.mu.Unlock()
					app.QueueUpdateDraw(updateHistoryList)
				}()
				if err := rerunTarget.run(rerunChan); err != nil {
					app.QueueUpdateDraw(func() {
						textView.SetText(fmt.Sprintf("Rerun failed: %v", err))
					})
				}
			}()
			return nil
		}
		return event
	})

	treeView.SetChangedFunc(func(node *tview.TreeNode) {
		viewLog(node, textView, searchQuery, -1)
	})

	// Process initial events
	go func() {
		processEvent := func(te collector.TestEvent) {
			if te.IsRootEvent() {
				return
			}
			initialHistory.mu.Lock()
			initialHistory.TestCases[te.Test] = append(initialHistory.TestCases[te.Test], te)
			testName := te.Test
			events := make([]collector.TestEvent, len(initialHistory.TestCases[te.Test]))
			copy(events, initialHistory.TestCases[te.Test])
			initialHistory.mu.Unlock()
			app.QueueUpdateDraw(func() {
				updateNode(initialHistory.Root, initialHistory.NodeMap, testName, events, spinnerFrames[spinnerFrame.Load()])
				if historyMgr.Current() == initialHistory {
					viewLog(treeView.GetCurrentNode(), textView, searchQuery, -1)
				}
			})
		}

		for {
			select {
			case te := <-eventChan:
				processEvent(te)
			case <-doneChan:
				// Drain remaining events before finishing
				for {
					select {
					case te := <-eventChan:
						processEvent(te)
					default:
						initialHistory.mu.Lock()
						initialHistory.State = stateFromTestResult(initialHistory.TestCases)
						initialHistory.mu.Unlock()
						app.QueueUpdateDraw(updateHistoryList)
						return
					}
				}
			}
		}
	}()

	// Layout: Left panel (History + Tests), Right panel (Log)
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(historyList, 0, 1, false).
		AddItem(treeView, 0, 3, true)

	mainFlex := tview.NewFlex().
		AddItem(leftPanel, 0, 1, true).
		AddItem(logPanel, 0, 2, false)

	footer := tview.NewFlex().AddItem(usageView, 0, 1, false)
	appFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainFlex, 0, 1, true).
		AddItem(footer, 1, 0, false)

	if err := app.SetRoot(appFlex, true).Run(); err != nil {
		panic(err)
	}
}

// rerunTarget holds information needed to rerun a test or package
type rerunTarget struct {
	historyName string
	run         func(chan<- collector.TestEvent) error
}

// parseRerunTarget extracts rerun information from a node reference
func parseRerunTarget(ref interface{}) *rerunTarget {
	if ref == nil {
		return nil
	}

	switch v := ref.(type) {
	case string:
		// Package node
		pkg := v
		return &rerunTarget{
			historyName: fmt.Sprintf("Rerun: pkg %s", lastPathComponent(pkg)),
			run: func(ch chan<- collector.TestEvent) error {
				return collector.RunPackage(pkg, ch)
			},
		}
	case []collector.TestEvent:
		// Test node
		if len(v) == 0 {
			return nil
		}
		pkg := v[0].Package
		testName := v[0].Test
		return &rerunTarget{
			historyName: fmt.Sprintf("Rerun: %s", lastPathComponent(testName)),
			run: func(ch chan<- collector.TestEvent) error {
				return collector.RunTest(pkg, testName, ch)
			},
		}
	default:
		return nil
	}
}

// lastPathComponent returns the last component of a slash-separated path
func lastPathComponent(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func updateNode(root *tview.TreeNode, nodeMap map[string]*tview.TreeNode, testName string, events []collector.TestEvent, spinnerIcon string) {
	if len(events) == 0 {
		return
	}

	// Get package name and create package node
	pkg := events[0].Package
	pkgNode, exists := nodeMap["pkg:"+pkg]
	if !exists {
		pkgNode = tview.NewTreeNode("📦 " + lastPathComponent(pkg))
		pkgNode.SetExpanded(true)
		pkgNode.SetColor(tcell.ColorBlue)
		pkgNode.SetReference(pkg) // Store full package path for rerun
		nodeMap["pkg:"+pkg] = pkgNode
		root.AddChild(pkgNode)
	}

	// Build test hierarchy under package node
	parts := strings.Split(testName, "/")
	parent := pkgNode

	for i, part := range parts {
		path := pkg + ":" + strings.Join(parts[:i+1], "/")
		node, exists := nodeMap[path]
		if !exists {
			node = tview.NewTreeNode(part)
			node.SetExpanded(true)
			nodeMap[path] = node
			parent.AddChild(node)
		}
		parent = node
	}

	statusIcon, color, elapsed := resolveTestStatus(events, spinnerIcon)

	parent.SetReference(events)

	testDisplayName := parts[len(parts)-1]
	expandIcon := getExpandIcon(parent)
	text := formatNodeText(expandIcon, statusIcon, testDisplayName, elapsed)
	parent.SetText(text).SetColor(color)
}

// formatNodeText formats the display text for a tree node
func formatNodeText(expandIcon, statusIcon, name string, elapsed float64) string {
	if elapsed > 0 {
		return fmt.Sprintf("%s%s %s [%.3fs]", expandIcon, statusIcon, name, elapsed)
	}
	return fmt.Sprintf("%s%s %s", expandIcon, statusIcon, name)
}

// getExpandIcon returns the expand/collapse icon for a node based on its state
func getExpandIcon(node *tview.TreeNode) string {
	if len(node.GetChildren()) == 0 {
		return ""
	}
	if node.IsExpanded() {
		return "▼ "
	}
	return "▶ "
}

// updateNodeExpandIcon updates only the expand/collapse indicator of a node
func updateNodeExpandIcon(node *tview.TreeNode) {
	if node == nil {
		return
	}

	text := node.GetText()
	if text == "" || text == "." {
		return
	}

	// Remove existing expand icon and add the current one
	text = strings.TrimPrefix(text, "▼ ")
	text = strings.TrimPrefix(text, "▶ ")
	node.SetText(getExpandIcon(node) + text)
}

func getTestEvent(node *tview.TreeNode) []collector.TestEvent {
	events, _ := node.GetReference().([]collector.TestEvent)
	return events
}

func viewLog(node *tview.TreeNode, textView *tview.TextView, searchQuery string, currentMatch int) {
	if node == nil || node.GetText() == "." {
		textView.SetText("select testcase")
		return
	}

	var builder strings.Builder
	for _, event := range getTestEvent(node) {
		builder.WriteString(event.Output)
	}

	text := builder.String()
	if searchQuery != "" {
		text = highlightMatches(text, searchQuery, currentMatch)
	}
	textView.SetText(text)
}

// highlightMatches highlights all occurrences of query in text (case-insensitive)
// currentMatch indicates which match (0-indexed) should be highlighted as current (-1 for none)
func highlightMatches(text, query string, currentMatch int) string {
	if query == "" {
		return text
	}

	// Use case-insensitive search with proper UTF-8 handling
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	queryLen := len(lowerQuery)

	var result strings.Builder
	lastEnd := 0
	matchIndex := 0
	textPos := 0 // Position in original text

	for {
		idx := strings.Index(lowerText[lastEnd:], lowerQuery)
		if idx == -1 {
			result.WriteString(text[textPos:])
			break
		}

		// Find corresponding position in original text
		// by counting the same number of bytes in lowercased text
		matchStartLower := lastEnd + idx
		matchEndLower := matchStartLower + queryLen

		// Map positions from lowercased to original text
		origStart := mapLowerToOrigPos(text, lowerText, matchStartLower)
		origEnd := mapLowerToOrigPos(text, lowerText, matchEndLower)

		// Add text before match
		result.WriteString(text[textPos:origStart])

		// Add highlighted match
		if matchIndex == currentMatch {
			// Current match: orange background (vim-like IncSearch)
			result.WriteString("[#000000:#ff8800:b]")
		} else {
			// Other matches: yellow background (vim-like Search)
			result.WriteString("[#000000:#ffff00:b]")
		}
		result.WriteString(text[origStart:origEnd])
		result.WriteString("[-:-:-]")

		lastEnd = matchEndLower
		textPos = origEnd
		matchIndex++
	}

	return result.String()
}

// mapLowerToOrigPos maps a byte position in lowercased text to the original text
// This handles cases where lowercasing changes byte length (e.g., some Unicode chars)
func mapLowerToOrigPos(orig, lower string, lowerPos int) int {
	if lowerPos <= 0 {
		return 0
	}
	if lowerPos >= len(lower) {
		return len(orig)
	}

	// For ASCII-only text (common case), positions are the same
	// Check if we can use fast path
	if len(orig) == len(lower) {
		return lowerPos
	}

	// Slow path: iterate through runes to find correct position
	origPos := 0
	lowerIdx := 0
	for _, r := range orig {
		if lowerIdx >= lowerPos {
			break
		}
		lowerR := []rune(strings.ToLower(string(r)))
		for _, lr := range lowerR {
			lowerIdx += len(string(lr))
		}
		origPos += len(string(r))
	}
	return origPos
}

// resolveTestStatus determines the status icon, color, and elapsed time from test events
func resolveTestStatus(events []collector.TestEvent, spinnerIcon string) (statusIcon string, color tcell.Color, elapsed float64) {
	statusIcon = "⧗"
	color = tcell.ColorDefault

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
			statusIcon = spinnerIcon
			color = tcell.ColorYellow
		case collector.ActionSkip:
			statusIcon = "⏭"
			color = tcell.ColorDarkCyan
		case collector.ActionStart:
			statusIcon = "⧗"
			color = tcell.ColorGray
		}
	}
	return statusIcon, color, elapsed
}

// isTestRunning checks if the test is still running based on the last terminal action
func isTestRunning(events []collector.TestEvent) bool {
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Action {
		case collector.ActionPass, collector.ActionFail, collector.ActionSkip:
			return false
		case collector.ActionRun:
			return true
		}
	}
	return false
}

// hasFailedTest checks if any test in the test cases has failed
func hasFailedTest(testCases TestCaseMap) bool {
	for _, events := range testCases {
		for _, te := range events {
			if te.Action == collector.ActionFail {
				return true
			}
		}
	}
	return false
}

// stateFromTestResult returns the appropriate history state based on test results
func stateFromTestResult(testCases TestCaseMap) HistoryState {
	if hasFailedTest(testCases) {
		return StateFailed
	}
	return StateCompleted
}

// historyStateSuffix returns the display suffix for a history state
func historyStateSuffix(state HistoryState, spinnerIcon string) string {
	switch state {
	case StateRunning:
		return " " + spinnerIcon
	case StateCompleted:
		return " ✓"
	case StateFailed:
		return " ✗"
	default:
		return ""
	}
}
