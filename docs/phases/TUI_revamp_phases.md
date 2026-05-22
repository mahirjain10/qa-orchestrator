# TUI Revamp — Phased Implementation Plan

> **Status:** Approved design document
> **Created:** 2026-05-20
> **Scope:** Complete redesign of the QA Orchestrator TUI from quadrant-based dashboard to sidebar+main operator console
> **Target look/feel:** k9s, lazygit, lazydocker, Claude Code, Warp

---

## Current State Summary

The existing TUI (`apps/tui/`) uses a rigid 2x2 quadrant layout with polling-based updates, no viewport components, duplicate style definitions across files, no scrolling, and a slot-based navigation model that creates cognitive overload. All panels have equal visual weight, traces are truncated to 8 events, and the footer is a 120+ character wall of text.

## Target State

A sidebar+main layout with event-driven updates, `bubbles/viewport` for all scrollable content, a unified design system, contextual key bindings, mode-based navigation, and a clean information hierarchy that prioritizes operator awareness over debug visibility.

---

## Critical Rules for Implementing Agents

1. **Read this document fully before starting any phase.**
2. **Each phase must compile successfully before the next begins.**
3. **Run existing tests after each phase. Fix any breakage.**
4. **Do NOT skip phases or merge scopes.**
5. **Do NOT modify another agent's completed phase without explicit permission.**
6. **After completing a phase, update `docs/CURRENT.md` and write a run summary.**
7. **If a phase reveals a blocking issue, stop and report — do not improvise.**

---

# PHASE 1 — Design System Unification

**Goal:** Single source of truth for all colors, styles, spacing, and typography. Eliminate duplicate style definitions across `campaign_list.go`, `flow_status.go`, `run_panel.go`, `trace_panel.go`, and `main.go`.

**Files affected:**
- CREATE `apps/tui/internal/style/theme.go`
- MODIFY `apps/tui/internal/components/campaign_list.go`
- MODIFY `apps/tui/internal/components/flow_status.go`
- MODIFY `apps/tui/internal/components/run_panel.go`
- MODIFY `apps/tui/internal/components/trace_panel.go`
- MODIFY `apps/tui/internal/screens/main.go`

**What to do:**

1. Create `apps/tui/internal/style/theme.go` with ALL color constants and pre-computed Lip Gloss styles:

```go
package style

import "github.com/charmbracelet/lipgloss"

// Color palette
const (
    Cyan    = lipgloss.Color("86")
    Green   = lipgloss.Color("76")
    Red     = lipgloss.Color("204")
    Yellow  = lipgloss.Color("228")
    Blue    = lipgloss.Color("75")
    Orange  = lipgloss.Color("208")
    Pink    = lipgloss.Color("205")
    Gray    = lipgloss.Color("245")
    DimGray = lipgloss.Color("241")
    Border  = lipgloss.Color("240")
    BgDark  = lipgloss.Color("235")
    BgSel   = lipgloss.Color("237")
    Text    = lipgloss.Color("252")
    TextSel = lipgloss.Color("229")
    BrightGreen = lipgloss.Color("82")
    BrightYellow = lipgloss.Color("214")
    Green46 = lipgloss.Color("46")
)

// Status indicator styles (single source of truth)
var (
    StatusRunning = lipgloss.NewStyle().Foreground(Blue).Bold(true)
    StatusPassed  = lipgloss.NewStyle().Foreground(Green)
    StatusFailed  = lipgloss.NewStyle().Foreground(Red)
    StatusPaused  = lipgloss.NewStyle().Foreground(Yellow)
    StatusPending = lipgloss.NewStyle().Foreground(Gray)
    StatusCancelled = lipgloss.NewStyle().Foreground(DimGray)
    StatusRetrying = lipgloss.NewStyle().Foreground(Yellow)
)

// Layout styles
var (
    Header       = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
    ViewTitle    = lipgloss.NewStyle().Foreground(Pink).Bold(true)
    Section      = lipgloss.NewStyle().Foreground(Gray).Bold(true)
    Normal       = lipgloss.NewStyle().Foreground(Text)
    Dim          = lipgloss.NewStyle().Foreground(DimGray)
    Selected     = lipgloss.NewStyle().Foreground(TextSel).Background(BgSel)
    SelectedBold = lipgloss.NewStyle().Foreground(Cyan).Bold(true).Background(BgDark)
    Help         = lipgloss.NewStyle().Foreground(DimGray)
    Msg          = lipgloss.NewStyle().Foreground(Cyan)
)

// Border styles
var (
    ActiveBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Cyan).Bold(true)
    InactiveBorder = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Border)
    PanelBorder    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Border)
    FocusBorder    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Cyan).Bold(true)
    ModalBorder    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Cyan)
    SidebarBorder  = lipgloss.NewStyle().Border(lipgloss.Border{Right: "│"}).BorderForeground(Border)
)

// Trace status helpers
func TraceStatusChar(s string) string {
    switch s {
    case "success":
        return "✓"
    case "failed":
        return "✗"
    case "skipped":
        return "○"
    default:
        return "·"
    }
}

func TraceStatusStyle(s string) lipgloss.Style {
    switch s {
    case "success":
        return lipgloss.NewStyle().Foreground(BrightGreen)
    case "failed":
        return lipgloss.NewStyle().Foreground(Red)
    case "skipped":
        return lipgloss.NewStyle().Foreground(Gray)
    default:
        return lipgloss.NewStyle().Foreground(DimGray)
    }
}
```

2. In each component file, DELETE all `var()` style blocks and replace imports to use `style.*`.

3. In `main.go`, DELETE all `var()` style blocks (lines 22-76) and replace with `style.*` references.

4. Verify: No component file should define its own colors. All styling goes through `style/theme.go`.

**Verification:** `go build ./...` succeeds. Visual output is identical or improved.

---

# PHASE 2 — Utility Functions

**Goal:** Create shared utility functions for truncation, safe width calculation, and time formatting. Eliminate inline truncation logic scattered across components.

**Files affected:**
- CREATE `apps/tui/internal/util/truncate.go`
- CREATE `apps/tui/internal/util/truncate_test.go`
- MODIFY component files to use `util.Truncate()` and `util.TruncateMiddle()`

**What to do:**

```go
// util/truncate.go
package util

func Truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    if maxLen <= 3 {
        return s[:maxLen]
    }
    return s[:maxLen-3] + "..."
}

func TruncateMiddle(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    if maxLen <= 7 {
        return s[:maxLen]
    }
    half := (maxLen - 3) / 2
    return s[:half] + "..." + s[len(s)-(maxLen-half-3):]
}

func SafeWidth(w, min int) int {
    if w < min {
        return min
    }
    return w
}

func FormatDuration(d time.Duration) string {
    if d < time.Minute {
        return d.Round(time.Second).String()
    }
    return d.Round(time.Second).String()
}
```

**Verification:** `go test ./apps/tui/internal/util/...` passes. `go build ./...` succeeds.

---

# PHASE 3 — State Architecture Refactor

**Goal:** Remove `AppState` (with its unnecessary mutexes). Merge domain state and UI state into a single Bubble Tea model. Bubble Tea's update loop is single-threaded — mutexes are not needed.

**Files affected:**
- DELETE `apps/tui/internal/state/app.go` (or deprecate)
- REWRITE `apps/tui/internal/screens/main.go` — new model struct
- MODIFY `apps/tui/internal/screens/handlers.go` — keep as-is (business logic)

**What to do:**

Replace the current `MainScreen` struct:

```go
type MainScreen struct {
    // Stores (injected, never change)
    sessionStore    *session.SessionStore
    traceStore      *trace.TraceStore
    artifactStore   *artifact.ArtifactStore
    reportGenerator *reporting.ReportGenerator

    // Domain state
    runs      map[string]*types.Session
    currentRun *types.Session
    traces    []*types.TraceEvent
    artifacts []*artifact.Artifact

    // UI state
    activeView   View
    sidebarFocus bool
    width        int
    height       int
    msg          string
    msgTime      time.Time
    loading      bool

    // Sub-models
    campaignList  *components.CampaignListModel
    flowStatus    *components.FlowStatusModel
    tracePanel    *components.TracePanelModel
    artifactPanel *components.ArtifactPanelModel
    spinner       spinner.Model
    steeringInput textinput.Model
    steeringMode  bool
    reportView    string
}
```

Key changes:
1. Remove `state.AppState` dependency entirely
2. Remove all `sync.RWMutex` usage — Bubble Tea is single-threaded
3. `CurrentRunID` becomes `currentRun *types.Session` (the actual object, not just an ID)
4. `CurrentView`, `SelectedIdx` removed — replaced by `activeView` and `sidebarFocus`
5. `RefreshSessions()` becomes an async command (see Phase 4)

**Verification:** `go build ./...` succeeds. All existing tests pass.

---

# PHASE 4 — Async Event System (Replace Polling)

**Goal:** Replace the 1-second polling tick with async commands that fetch data in background goroutines and return messages to the update loop.

**Files affected:**
- CREATE `apps/tui/internal/screens/messages.go`
- CREATE `apps/tui/internal/screens/commands.go`
- MODIFY `apps/tui/internal/screens/main.go` — Update() and Init()

**What to do:**

1. Create message types:

```go
// messages.go
type sessionsLoadedMsg struct{ sessions []*types.Session }
type runLoadedMsg struct{ run *types.Session }
type tracesLoadedMsg struct{ traces []*types.TraceEvent }
type artifactsLoadedMsg struct{ artifacts []*artifact.Artifact }
type reportLoadedMsg struct{ report string }
type tickMsg time.Time
type errMsg struct{ err error }
```

2. Create command functions that return `tea.Cmd`:

```go
// commands.go
func fetchSessionsCmd(store *session.SessionStore) tea.Cmd {
    return func() tea.Msg {
        sessions, err := store.List()
        if err != nil {
            return errMsg{err}
        }
        return sessionsLoadedMsg{sessions}
    }
}

func fetchRunCmd(store *session.SessionStore, runID string) tea.Cmd {
    return func() tea.Msg {
        if runID == "" {
            return nil
        }
        run, err := store.Get(runID)
        if err != nil {
            return errMsg{err}
        }
        return runLoadedMsg{run}
    }
}

func fetchTracesCmd(store *trace.TraceStore, runID string) tea.Cmd {
    return func() tea.Msg {
        if runID == "" || store == nil {
            return nil
        }
        traces, err := store.GetRecent(runID, 50)
        if err != nil {
            return errMsg{err}
        }
        return tracesLoadedMsg{traces}
    }
}

func fetchArtifactsCmd(store *artifact.ArtifactStore, runID string) tea.Cmd {
    return func() tea.Msg {
        if runID == "" || store == nil {
            return nil
        }
        artifacts, err := store.GetByRunID(runID)
        if err != nil {
            return errMsg{err}
        }
        return artifactsLoadedMsg{artifacts}
    }
}

func refreshCmd(runID string, stores stores) tea.Cmd {
    return tea.Batch(
        fetchRunCmd(stores.sessionStore, runID),
        fetchTracesCmd(stores.traceStore, runID),
        fetchArtifactsCmd(stores.artifactStore, runID),
    )
}

func startRefreshTicker(runID string, stores stores) tea.Cmd {
    if runID == "" {
        return nil
    }
    return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}
```

3. Rewrite `Init()`:

```go
func (m *MainScreen) Init() tea.Cmd {
    return tea.Batch(
        fetchSessionsCmd(m.sessionStore),
        tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
            return tickMsg(t)
        }),
    )
}
```

4. Rewrite `Update()` to handle async messages:

```go
case tickMsg:
    runID := ""
    if m.currentRun != nil {
        runID = m.currentRun.RunID
    }
    return m, tea.Batch(
        refreshCmd(runID, m.stores()),
        startRefreshTicker(runID, m.stores()),
    )

case sessionsLoadedMsg:
    for _, s := range msg.sessions {
        m.runs[s.RunID] = s
    }
    m.campaignList.SetCampaigns(m.campaignNames())
    return m, nil

case runLoadedMsg:
    m.runs[msg.run.RunID] = msg.run
    if m.currentRun != nil && m.currentRun.RunID == msg.run.RunID {
        m.currentRun = msg.run
        m.runPanel.SetSession(msg.run)
        m.flowStatus.SetFlows(msg.run.Flows)
    }
    return m, nil

case tracesLoadedMsg:
    m.traces = msg.traces
    m.tracePanel.SetEvents(msg.traces)
    return m, nil

case artifactsLoadedMsg:
    m.artifacts = msg.artifacts
    m.artifactPanel.SetArtifacts(msg.artifacts)
    return m, nil

case errMsg:
    m.setMsg("Error: " + msg.err.Error())
    return m, nil
```

**Verification:** `go build ./...` succeeds. UI no longer freezes during data fetch. Ticker runs at 2s instead of 1s.

---

# PHASE 5 — Layout System: Sidebar + Main Content

**Goal:** Replace the 2x2 quadrant system (`quadrants [4]ComponentID`, `activeSlot`, `maximized`, `maximizedSlot`) with a sidebar + main content layout.

**Files affected:**
- REWRITE `apps/tui/internal/screens/main.go` — View() method
- MODIFY `apps/tui/internal/screens/main.go` — Update() method (remove slot keys)
- DELETE quadrant-related fields from MainScreen struct

**What to do:**

1. Remove from `MainScreen`:
   - `quadrants [4]ComponentID`
   - `activeSlot int`
   - `maximized bool`
   - `maximizedSlot int`
   - `focusColorForSlot()` method

2. Add to `MainScreen`:
   - `activeView View` (Dashboard, Flows, Traces, Report)
   - `sidebarFocus bool`

3. Add View type:

```go
type View string

const (
    ViewDashboard View = "dashboard"
    ViewFlows     View = "flows"
    ViewTraces    View = "traces"
    ViewReport    View = "report"
)
```

4. New `View()` method structure:

```go
func (m *MainScreen) View() string {
    if m.width == 0 || m.height == 0 {
        return "Initializing..."
    }

    if m.width < 80 || m.height < 24 {
        return style.Dim.Render("Terminal too small. Minimum: 80x24")
    }

    sidebar := m.renderSidebar()
    mainContent := m.renderMainContent()

    sidebarWidth := 24
    contentWidth := m.width - sidebarWidth - 2 // border + padding
    contentHeight := m.height - 5 // header + status + borders

    body := lipgloss.JoinHorizontal(lipgloss.Top,
        style.SidebarBorder.Width(sidebarWidth).Height(contentHeight).Render(sidebar),
        lipgloss.NewStyle().Width(contentWidth).Height(contentHeight).Render(mainContent),
    )

    return lipgloss.JoinVertical(lipgloss.Left,
        m.renderHeader(),
        body,
        m.renderStatusBar(),
    )
}
```

5. Sidebar rendering:

```go
func (m *MainScreen) renderSidebar() string {
    views := []struct {
        id    View
        label string
        key   string
    }{
        {ViewDashboard, "Dashboard", "1"},
        {ViewFlows, "Flows", "2"},
        {ViewTraces, "Traces", "3"},
        {ViewReport, "Report", "4"},
    }

    lines := []string{
        style.Section.Render("  VIEWS"),
        "",
    }

    for _, v := range views {
        var line string
        if v.id == m.activeView && m.sidebarFocus {
            line = style.SelectedBold.Render(" " + v.key + " " + v.label + " ")
        } else if v.id == m.activeView {
            line = style.Normal.Bold(true).Render(" " + v.key + " " + v.label)
        } else {
            line = style.Dim.Render(" " + v.key + " " + v.label)
        }
        lines = append(lines, line)
    }

    // Run info section
    if m.currentRun != nil {
        lines = append(lines, "")
        lines = append(lines, style.Section.Render("  RUN"))
        lines = append(lines, style.Dim.Render("  " + m.currentRun.RunID[:8]))
        statusStyle := statusStyleForRun(m.currentRun.Status)
        lines = append(lines, statusStyle.Render("  " + string(m.currentRun.Status)))
    }

    return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
```

6. Main content rendering:

```go
func (m *MainScreen) renderMainContent() string {
    switch m.activeView {
    case ViewDashboard:
        return m.renderDashboardView()
    case ViewFlows:
        return m.renderFlowsView()
    case ViewTraces:
        return m.renderTracesView()
    case ViewReport:
        return m.renderReportView()
    default:
        return style.Dim.Render("  Unknown view")
    }
}
```

7. Update key handling in `Update()`:

REMOVE these key cases:
- `"tab"` (slot cycling) — replace with sidebar/content focus toggle
- `"left"`, `"right"` (slot navigation) — replace with view switching
- `"p"` (cycle component) — DELETE
- `"w"` (swap slots) — DELETE
- `"0"`, `"1"`, `"2"`, `"3"` (slot jump) — replace with `"1"`-`"4"` view jump
- `"m"` (maximize) — DELETE

ADD these key cases:
```go
case "1":
    m.activeView = ViewDashboard
    m.msg = "Dashboard view"

case "2":
    m.activeView = ViewFlows
    m.msg = "Flows view"

case "3":
    m.activeView = ViewTraces
    m.msg = "Traces view"

case "4":
    m.activeView = ViewReport
    m.msg = "Report view"

case "tab":
    m.sidebarFocus = !m.sidebarFocus
    m.msg = map[bool]string{true: "Sidebar focused", false: "Content focused"}[m.sidebarFocus]

case "up", "k":
    if m.sidebarFocus {
        m.cycleView(-1)
    } else {
        m.handleContentUp()
    }

case "down", "j":
    if m.sidebarFocus {
        m.cycleView(1)
    } else {
        m.handleContentDown()
    }
```

**Verification:** `go build ./...` succeeds. Layout renders as sidebar + main. All old slot keys removed.

---

# PHASE 6 — Dashboard View

**Goal:** Create the default view showing run summary + flow timeline. This replaces the quadrant-based dashboard.

**Files affected:**
- MODIFY `apps/tui/internal/screens/main.go` — add `renderDashboardView()`
- MODIFY `apps/tui/internal/components/run_panel.go` — adapt for dashboard
- MODIFY `apps/tui/internal/components/flow_status.go` — adapt for dashboard

**What to do:**

Create `renderDashboardView()`:

```go
func (m *MainScreen) renderDashboardView() string {
    if m.currentRun == nil {
        return m.renderCampaignSelector()
    }

    // Top: run summary
    summary := m.renderRunSummary()

    // Bottom: flow timeline
    flows := m.renderFlowTimeline()

    return lipgloss.JoinVertical(lipgloss.Left,
        summary,
        "",
        flows,
    )
}
```

Run summary (compact version of run panel):

```go
func (m *MainScreen) renderRunSummary() string {
    sess := m.currentRun
    statusStyle := statusStyleForRun(sess.Status)

    var spinnerStr string
    if sess.Status == types.RunStateRunning {
        spinnerStr = m.spinner.View() + " "
    }

    lines := []string{
        style.ViewTitle.Render(" Run Summary "),
        "",
        fmt.Sprintf("  %s%s", spinnerStr, statusStyle.Render(string(sess.Status))),
        style.Dim.Render("  Campaign: " + sess.CampaignName),
        style.Dim.Render("  Agent:    " + sess.CurrentAgent),
        style.Dim.Render("  Flow:     " + sess.CurrentFlowID),
    }

    // Flow counts
    var running, passed, failed int
    for _, f := range sess.Flows {
        switch f.Status {
        case types.FlowStateRunning:
            running++
        case types.FlowStatePassed:
            passed++
        case types.FlowStateFailed:
            failed++
        }
    }

    counts := fmt.Sprintf("  %d flows | %s %d  %s %d  %s %d",
        len(sess.Flows),
        style.StatusRunning.Render(fmt.Sprintf("R:%d", running)),
        style.StatusPassed.Render(fmt.Sprintf("P:%d", passed)),
        style.StatusFailed.Render(fmt.Sprintf("F:%d", failed)),
    )
    lines = append(lines, counts)

    content := lipgloss.JoinVertical(lipgloss.Left, lines...)
    return style.PanelBorder.Width(m.contentWidth()).Padding(0, 1).Render(content)
}
```

Flow timeline (compact flow status):

```go
func (m *MainScreen) renderFlowTimeline() string {
    if m.currentRun == nil || len(m.currentRun.Flows) == 0 {
        return style.Dim.Render("  No flows")
    }

    lines := []string{
        style.Section.Render("  Flows"),
        "",
    }

    for i, f := range m.currentRun.Flows {
        statusStyle := statusStyleForFlow(f.Status)
        statusChar := statusCharForFlow(f.Status)

        indicator := "  "
        if i == m.flowStatus.GetSelected() && !m.sidebarFocus {
            indicator = style.SelectedBold.Render(" ▶ ")
        }

        row := fmt.Sprintf("%s%s  %s",
            indicator,
            statusStyle.Render(statusChar),
            f.FlowID,
        )
        lines = append(lines, row)
    }

    return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
```

**Verification:** `go build ./...` succeeds. Dashboard shows run summary + flow list.

---

# PHASE 7 — Viewport Integration for Traces

**Goal:** Replace manual truncation (`maxEvents: 50`, `recentEvents = events[len(events)-8:]`) with `bubbles/viewport` for proper scrolling.

**Files affected:**
- MODIFY `apps/tui/internal/components/trace_panel.go` — add viewport
- MODIFY `apps/tui/internal/screens/main.go` — `renderTracesView()`
- MODIFY `go.mod` — ensure `bubbles/viewport` is imported

**What to do:**

1. Update `TracePanelModel`:

```go
import "github.com/charmbracelet/bubbles/viewport"

type TracePanelModel struct {
    events     []*types.TraceEvent
    selected   int
    viewport   viewport.Model
    followTail bool
}

func NewTracePanelModel() *TracePanelModel {
    vp := viewport.New(80, 20)
    return &TracePanelModel{
        events:     []*types.TraceEvent{},
        selected:   0,
        viewport:   vp,
        followTail: true,
    }
}

func (m *TracePanelModel) SetEvents(events []*types.TraceEvent) {
    m.events = events
    if m.selected >= len(m.events) {
        m.selected = max(0, len(m.events)-1)
    }
    m.updateViewportContent()
    if m.followTail {
        m.viewport.GotoBottom()
    }
}

func (m *TracePanelModel) SetSize(width, height int) {
    m.viewport.Width = width
    m.viewport.Height = height
    m.updateViewportContent()
}

func (m *TracePanelModel) updateViewportContent() {
    var lines []string

    // Column headers
    lines = append(lines, style.Section.Render("  TIME     S  TYPE              ACTION"))
    lines = append(lines, style.Dim.Render("  " + strings.Repeat("─", 62)))

    for i := len(m.events) - 1; i >= 0; i-- {
        e := m.events[i]
        timeStr := e.Timestamp.Format("15:04:05")
        statusChar := style.TraceStatusChar(string(e.Status))
        statusSt := style.TraceStatusStyle(string(e.Status))
        typeStr := util.Truncate(string(e.EventType), 18)
        actionStr := util.Truncate(e.Action, 40)

        cursor := "  "
        if i == m.selected {
            cursor = style.SelectedBold.Render(" ▶ ")
        }

        row := fmt.Sprintf("%s%s  %s  %-18s  %s",
            cursor,
            style.Dim.Render(timeStr),
            statusSt.Render(statusChar),
            typeStr,
            actionStr,
        )
        lines = append(lines, row)
    }

    m.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// Update handles viewport scroll keys
func (m *TracePanelModel) Update(msg tea.Msg) {
    var cmd tea.Cmd
    m.viewport, cmd = m.viewport.Update(msg)
    // Return cmd if needed
}
```

2. In `renderTracesView()`:

```go
func (m *MainScreen) renderTracesView() string {
    m.tracePanel.SetSize(m.contentWidth(), m.contentHeight()-2)
    return m.tracePanel.viewport.View()
}
```

3. Add follow-tail toggle key:

```go
case "f":
    if m.activeView == ViewTraces {
        m.tracePanel.followTail = !m.tracePanel.followTail
        m.msg = fmt.Sprintf("Follow tail: %v", m.tracePanel.followTail)
    }
```

**Verification:** `go build ./...` succeeds. Traces scroll properly. No truncation to 8 events.

---

# PHASE 8 — Flows View with Detail Panel

**Goal:** Full flow table with expandable detail. Shows all flow fields: mode, priority, status, started, duration, retry count, error message.

**Files affected:**
- MODIFY `apps/tui/internal/screens/main.go` — add `renderFlowsView()`
- MODIFY `apps/tui/internal/components/flow_status.go` — add viewport + detail

**What to do:**

1. Add viewport to `FlowStatusModel`:

```go
import "github.com/charmbracelet/bubbles/viewport"

type FlowStatusModel struct {
    flows      []types.FlowRunState
    selected   int
    expanded   bool
    viewport   viewport.Model
}
```

2. `renderFlowsView()`:

```go
func (m *MainScreen) renderFlowsView() string {
    if m.currentRun == nil || len(m.currentRun.Flows) == 0 {
        return style.Dim.Render("  No flows")
    }

    lines := []string{
        style.ViewTitle.Render(" Flows "),
        "",
    }

    // Table header
    colFlow := util.SafeWidth(m.contentWidth()/3, 16)
    colMode := 10
    colPriority := 10
    colStatus := 12

    headerFmt := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds %%-%ds", colFlow, colMode, colPriority, colStatus)
    lines = append(lines, style.Section.Render(fmt.Sprintf(headerFmt, "Flow", "Mode", "Priority", "Status")))
    lines = append(lines, style.Dim.Render("  " + strings.Repeat("─", m.contentWidth()-4)))

    for i, f := range m.currentRun.Flows {
        statusStyle := statusStyleForFlow(f.Status)

        cursor := "  "
        if i == m.flowStatus.GetSelected() && !m.sidebarFocus {
            cursor = style.SelectedBold.Render(" ▶ ")
        }

        flowID := util.Truncate(f.FlowID, colFlow-4)

        row := fmt.Sprintf("%s%%-%ds %%-%ds %%-%ds %%-%ds", cursor, colFlow, colMode, colPriority, colStatus)
        line := fmt.Sprintf(row, flowID, string(f.Mode), string(f.Priority), statusStyle.Render(string(f.Status)))
        lines = append(lines, line)

        // Expanded detail
        if i == m.flowStatus.GetSelected() && m.flowStatus.expanded && !m.sidebarFocus {
            detail := m.renderFlowDetail(f)
            lines = append(lines, detail)
        }
    }

    content := lipgloss.JoinVertical(lipgloss.Left, lines...)
    return style.PanelBorder.Width(m.contentWidth()).Padding(0, 1).Render(content)
}

func (m *MainScreen) renderFlowDetail(f types.FlowRunState) string {
    lines := []string{
        style.Dim.Render("    ──────────────────────────────────────"),
    }

    if f.StartedAt != nil {
        lines = append(lines, style.Dim.Render("    Started:  "+f.StartedAt.Format("15:04:05")))
    }
    if f.FinishedAt != nil {
        lines = append(lines, style.Dim.Render("    Finished: "+f.FinishedAt.Format("15:04:05")))
        dur := f.FinishedAt.Sub(*f.StartedAt)
        lines = append(lines, style.Dim.Render("    Duration: "+dur.Round(time.Second).String()))
    }
    if f.RetryCount > 0 {
        lines = append(lines, style.StatusRetrying.Render(fmt.Sprintf("    Retries:  %d", f.RetryCount)))
    }
    if f.Error != "" {
        lines = append(lines, style.StatusFailed.Render("    Error:    "+util.Truncate(f.Error, 60)))
    }

    return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
```

3. Add expand/collapse keys:

```go
case "enter":
    if m.activeView == ViewFlows && !m.sidebarFocus {
        m.flowStatus.expanded = !m.flowStatus.expanded
    }

case "left", "h":
    if m.activeView == ViewFlows && m.flowStatus.expanded && !m.sidebarFocus {
        m.flowStatus.expanded = false
    }
```

**Verification:** `go build ./...` succeeds. Flows view shows table. Enter expands detail.

---

# PHASE 9 — Trace Filtering

**Goal:** Add filtering capability to the Traces view. Filter by text, severity, type, and flow ID.

**Files affected:**
- MODIFY `apps/tui/internal/components/trace_panel.go` — add filter state
- MODIFY `apps/tui/internal/screens/main.go` — add filter mode + keys

**What to do:**

1. Add filter state to `TracePanelModel`:

```go
type TraceFilter struct {
    text        string
    showFailed  bool
    flowID      string
    eventType   string
}

type TracePanelModel struct {
    // ... existing fields
    filter     TraceFilter
    filterMode bool
    filterInput textinput.Model
}
```

2. Add filtered event list:

```go
func (m *TracePanelModel) filteredEvents() []*types.TraceEvent {
    if m.filter.text == "" && !m.filter.showFailed && m.filter.flowID == "" && m.filter.eventType == "" {
        return m.events
    }

    var filtered []*types.TraceEvent
    for _, e := range m.events {
        if m.filter.showFailed && e.Status != types.TraceStatusFailed {
            continue
        }
        if m.filter.flowID != "" && e.FlowID != m.filter.flowID {
            continue
        }
        if m.filter.eventType != "" && string(e.EventType) != m.filter.eventType {
            continue
        }
        if m.filter.text != "" && !strings.Contains(strings.ToLower(e.Action), strings.ToLower(m.filter.text)) {
            continue
        }
        filtered = append(filtered, e)
    }
    return filtered
}
```

3. Add filter keys:

```go
case "/":
    if m.activeView == ViewTraces && !m.sidebarFocus {
        m.tracePanel.filterMode = true
        m.tracePanel.filterInput.Focus()
        m.msg = "Filter traces (ESC to cancel)"
    }

case "s":
    if m.activeView == ViewTraces && !m.sidebarFocus && !m.steeringMode {
        m.tracePanel.filter.showFailed = !m.tracePanel.filter.showFailed
        m.msg = fmt.Sprintf("Show failed only: %v", m.tracePanel.filter.showFailed)
    }
```

**Verification:** `go build ./...` succeeds. `/` opens filter. `S` toggles failures-only.

---

# PHASE 10 — Status Bar + Contextual Help

**Goal:** Replace the 120+ character footer with a contextual status bar that shows relevant keys based on active view and state.

**Files affected:**
- MODIFY `apps/tui/internal/screens/main.go` — `renderStatusBar()` and `View()`

**What to do:**

1. New status bar:

```go
func (m *MainScreen) renderStatusBar() string {
    if m.height < 20 {
        return "" // Hide on short terminals
    }

    var left, right string

    // Left: run status
    if m.currentRun != nil {
        statusStyle := statusStyleForRun(m.currentRun.Status)
        left = statusStyle.Render(" " + string(m.currentRun.Status) + " ") +
            style.Dim.Render(" "+m.currentRun.RunID[:min(12, len(m.currentRun.RunID))])
    } else {
        left = style.Dim.Render(" IDLE")
    }

    // Right: contextual keys
    right = m.contextualKeys()

    // Message (if recent, within 5 seconds)
    var msgLine string
    if time.Since(m.msgTime) < 5*time.Second && m.msg != "" {
        msgLine = style.Msg.Render(" " + m.msg + " ")
    }

    bar := lipgloss.NewStyle().
        Background(style.BgDark).
        Width(m.width).
        Render(lipgloss.JoinHorizontal(lipgloss.Top,
            left,
            lipgloss.NewStyle().Width(max(0, m.width-len(left)-len(right)-20)).Render(""),
            right,
        ))

    if msgLine != "" {
        return lipgloss.JoinVertical(lipgloss.Left, msgLine, bar)
    }
    return bar
}

func (m *MainScreen) contextualKeys() string {
    switch m.activeView {
    case ViewDashboard:
        if m.currentRun != nil {
            return style.Dim.Render("space:pause  x:cancel  s:steer  ?:help")
        }
        return style.Dim.Render("enter:select  r:refresh  ?:help")
    case ViewTraces:
        return style.Dim.Render("/:filter  S:failures  F:follow  ?:help")
    case ViewFlows:
        return style.Dim.Render("enter:detail  r:retry  k:skip  ?:help")
    default:
        return style.Dim.Render("?:help")
    }
}
```

2. Remove the old footer from `View()`:

DELETE this line from `View()`:
```go
footer := helpStyle.Render("TAB/←→: switch slot │ 0-3: jump │ p: cycle │ w: swap │ m: maximize │ ↑↓ Navigate │ Enter: select │ Space: pause │ x: cancel │ s: steer │ q: quit")
```

**Verification:** `go build ./...` succeeds. Status bar shows contextual keys. Old footer gone.

---

# PHASE 11 — Campaign Selection Modal

**Goal:** When no run is active, show a modal overlay for campaign selection instead of a blank panel.

**Files affected:**
- MODIFY `apps/tui/internal/screens/main.go` — `renderCampaignSelector()` and `View()`

**What to do:**

```go
func (m *MainScreen) renderCampaignSelector() string {
    modalWidth := util.SafeWidth(m.width-20, 40)
    if modalWidth > 70 {
        modalWidth = 70
    }

    title := style.ViewTitle.Render(" Select a Campaign ")
    separator := strings.Repeat("─", modalWidth-4)

    var items []string
    sessions := m.state.GetSessions()
    for i, s := range sessions {
        prefix := "  "
        if i == m.campaignList.GetSelected() {
            prefix = style.SelectedBold.Render(" ▶ ")
        }
        items = append(items, prefix+s.CampaignName+" ("+s.RunID+")")
    }

    if len(items) == 0 {
        items = append(items, style.Dim.Render("  No campaigns found. Run with: ./app campaign.yaml"))
    }

    content := lipgloss.JoinVertical(lipgloss.Left,
        title,
        separator,
        strings.Join(items, "\n"),
        "",
        style.Dim.Render(" ↑↓ navigate  enter: select  q: quit"),
    )

    return style.ModalBorder.Width(modalWidth).Padding(1, 2).Render(content)
}
```

In `renderDashboardView()`, when `m.currentRun == nil`:

```go
func (m *MainScreen) renderDashboardView() string {
    if m.currentRun == nil {
        selector := m.renderCampaignSelector()
        // Center the modal
        padding := (m.contentWidth() - 70) / 2
        if padding < 0 {
            padding = 0
        }
        return strings.Repeat(" ", padding) + selector
    }
    // ... rest of dashboard
}
```

**Verification:** `go build ./...` succeeds. Modal shows when no run active.

---

# PHASE 12 — Responsive Behavior

**Goal:** Handle terminals of different sizes gracefully. Enforce minimums, collapse sidebar on narrow terminals, hide non-essential elements on short terminals.

**Files affected:**
- MODIFY `apps/tui/internal/screens/main.go` — `View()` method

**What to do:**

Add size checks at the top of `View()`:

```go
func (m *MainScreen) View() string {
    if m.width == 0 || m.height == 0 {
        return "Initializing..."
    }

    // Hard minimum
    if m.width < 80 {
        return style.StatusFailed.Render("  Terminal too narrow (min 80 columns). Current: " + fmt.Sprint(m.width))
    }
    if m.height < 20 {
        return style.StatusFailed.Render("  Terminal too short (min 20 rows). Current: " + fmt.Sprint(m.height))
    }

    // Adaptive sidebar width
    sidebarWidth := 24
    if m.width < 100 {
        sidebarWidth = 20
    }
    if m.width < 90 {
        sidebarWidth = 16
    }

    // Adaptive content height
    contentHeight := m.height - 5
    if m.height < 25 {
        contentHeight = m.height - 3 // Hide status bar hints
    }

    // ... rest of View() using sidebarWidth and contentHeight
}
```

Helper methods:

```go
func (m *MainScreen) contentWidth() int {
    sidebarWidth := 24
    if m.width < 100 {
        sidebarWidth = 20
    }
    if m.width < 90 {
        sidebarWidth = 16
    }
    return m.width - sidebarWidth - 4 // border + padding
}

func (m *MainScreen) contentHeight() int {
    if m.height < 25 {
        return m.height - 3
    }
    return m.height - 5
}
```

**Verification:** `go build ./...` succeeds. App shows error on tiny terminals. Sidebar shrinks on narrow terminals.

---

# PHASE 13 — Goroutine Shutdown + Clean Exit

**Goal:** Add cancellation path for the campaign goroutine. Allow the TUI to signal the engine to stop. Clean up browser runtime on exit.

**Files affected:**
- MODIFY `apps/tui/cmd/main.go`
- MODIFY `apps/tui/internal/screens/main.go` — add cancel command to engine

**What to do:**

1. In `main.go`, create a cancellable context for the campaign:

```go
func main() {
    // ... stores setup ...

    mainScreen := screens.NewMainScreenWithStores(sessionStore, traceStore, artifactStore)

    p := tea.NewProgram(mainScreen)

    if campaignPath != "" {
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        // Pass cancel function to mainScreen
        mainScreen.SetCancelFunc(cancel)

        go runCampaignWithContext(ctx, agentEngine, camp, sess.RunID, sessionStore)
    }

    if _, err := p.Run(); err != nil {
        os.Stderr.WriteString("Error running TUI: " + err.Error() + "\n")
        os.Exit(1)
    }
}

func runCampaignWithContext(ctx context.Context, eng *engine.AgentEngine, camp *sharedtypes.Campaign, runID string, sessionStore *session.SessionStore) {
    for _, flow := range camp.Flows {
        select {
        case <-ctx.Done():
            return
        default:
            result := eng.RunFlow(runID, flow)
            _ = result
        }
    }
}
```

2. In `MainScreen`, add cancel function:

```go
type MainScreen struct {
    // ... existing fields
    cancelFunc context.CancelFunc
}

func (m *MainScreen) SetCancelFunc(fn context.CancelFunc) {
    m.cancelFunc = fn
}
```

3. Handle `x` key to cancel:

```go
case "x":
    runID := m.getCurrentRunID()
    if runID != "" {
        // Update session state
        m.handlers.CancelRun(runID)
        // Signal goroutine
        if m.cancelFunc != nil {
            m.cancelFunc()
        }
        m.setMsg("Run cancelled")
    }
```

**Verification:** `go build ./...` succeeds. Pressing `x` stops the campaign goroutine. App exits cleanly.

---

# PHASE 14 — Visual Polish + Final Cleanup

**Goal:** Apply consistent spacing, fix any remaining visual issues, ensure all panels use the design system, and clean up dead code.

**What to do:**

1. **Audit all component files** — ensure NO inline color values remain. All must use `style.*`.

2. **Consistent padding** — all panels use `Padding(0, 1)` for inner content.

3. **Consistent borders** — all panels use `style.PanelBorder`.

4. **Remove dead code:**
   - `state.AppState` (if not already deleted)
   - `quadrants`, `activeSlot`, `maximized`, `maximizedSlot` fields
   - `focusColorForSlot()` method
   - Old footer rendering
   - Unused `View` constants in `state/app.go`

5. **Run all tests:**
   ```
   go test ./apps/tui/...
   go test ./packages/...
   ```

6. **Build and manual test:**
   ```
   go build -o qa-orchestrator ./apps/tui/cmd/
   ./qa-orchestrator campaigns/sample-campaign.yaml
   ```

7. **Verify:**
   - No terminal overflow at 80x24
   - No terminal overflow at 120x40
   - No terminal overflow at 200x60
   - Traces scroll properly
   - Flows expand/collapse
   - Status bar shows contextual keys
   - Campaign modal shows when no run
   - Sidebar navigation works
   - View switching works (1-4 keys)
   - Pause/resume/cancel work
   - Steering mode works
   - Clean exit on `q`

**Verification:** `go build ./...` succeeds. All tests pass. Manual testing confirms all features.

---

# Implementation Dependency Graph

```
Phase 1  (Design System)
    ↓
Phase 2  (Utilities)
    ↓
Phase 3  (State Refactor)
    ↓
Phase 4  (Async Events) ───┐
    ↓                        │
Phase 5  (Layout System) ───┤
    ↓                        │
Phase 6  (Dashboard)     ───┤
    ↓                        │  (can be done in parallel
Phase 7  (Viewport)      ───┤   after Phase 5)
    ↓                        │
Phase 8  (Flows View)    ───┤
    ↓                        │
Phase 9  (Filtering)     ───┤
    ↓                        │
Phase 10 (Status Bar)   ────┘
    ↓
Phase 11 (Campaign Modal)
    ↓
Phase 12 (Responsive)
    ↓
Phase 13 (Shutdown)
    ↓
Phase 14 (Polish + Cleanup)
```

**Parallelizable after Phase 5:** Phases 6-10 can be implemented in any order once the layout system (Phase 5) is in place. They touch different views and don't depend on each other.

**Must be sequential:** Phases 1-5 must be done in order because each builds on the previous.

**Must be last:** Phases 11-14 depend on everything before them being complete.

---

# Quick Reference: Key Changes per Phase

| Phase | Key Action | Risk Level |
|-------|-----------|------------|
| 1 | Create `style/theme.go`, replace all inline styles | Low |
| 2 | Create `util/truncate.go` | Low |
| 3 | Remove `AppState`, merge into model | Medium |
| 4 | Replace polling with async commands | Medium |
| 5 | Replace quadrant with sidebar+main | High |
| 6 | Create dashboard view | Low |
| 7 | Add viewport to traces | Medium |
| 8 | Create flows view with detail | Low |
| 9 | Add trace filtering | Low |
| 10 | Replace footer with contextual status bar | Low |
| 11 | Create campaign selection modal | Low |
| 12 | Add responsive sizing | Low |
| 13 | Add goroutine cancellation | Medium |
| 14 | Visual polish + cleanup | Low |

---

# Testing Checklist (After All Phases)

- [ ] `go build ./...` succeeds with no warnings
- [ ] `go test ./apps/tui/...` passes
- [ ] `go test ./packages/...` passes
- [ ] `go vet ./...` passes
- [ ] Terminal 80x24: renders correctly, no overflow
- [ ] Terminal 120x40: renders correctly, sidebar visible
- [ ] Terminal 200x60: renders correctly, full layout
- [ ] Terminal 70x20: shows "too small" error
- [ ] Campaign modal shows when no run active
- [ ] Dashboard shows run summary + flow list
- [ ] Flows view shows table, enter expands detail
- [ ] Traces view scrolls, filter works
- [ ] View switching (1-4) works
- [ ] Sidebar navigation (tab, ↑↓) works
- [ ] Pause/resume (space) works
- [ ] Cancel (x) works and stops goroutine
- [ ] Steering mode (s) works
- [ ] Clean exit (q) works
- [ ] No memory leaks after extended run
- [ ] No UI freezes during data fetch
