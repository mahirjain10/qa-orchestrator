# TUI and Execution Integration Refactoring Phases

This document outlines the phases required to fix the TUI, make it visually appealing, and properly integrate the execution engine so that campaigns can be launched and managed directly.

## Current Issues Identified
1. **Engine Not Integrated:** The TUI currently acts only as a passive state viewer. `apps/tui/cmd/main.go` does not instantiate or wire up the `AgentEngine`, so no campaigns actually execute when the TUI starts.
2. **Missing CLI Support:** Users cannot run campaigns via the CLI (e.g., `make run campaigns/autonomous-campaign-01.yaml`), because the binary does not parse arguments.
3. **Poor Visual Appeal:** The TUI lacks structural separation, clear panel borders, and modern layout aesthetics.
4. **Broken Navigation:** Pressing `f` to view flows or navigating through the views breaks the mental model or visual layout.

---

## Phase 1: CLI Execution & Engine Wiring
**Goal:** Allow users to pass a campaign file as an argument and actually run the engine.

1. **CLI Arguments in `main.go`:**
   - Update `apps/tui/cmd/main.go` to accept a campaign file path as a CLI argument.
   - Example usage: `qa-orchestrator campaigns/sample-autonomous.yaml`.
2. **Initialize Dependencies:**
   - Initialize `SessionStore`, `TraceStore`, and `ArtifactStore`.
   - Initialize `AgentEngine` (`NewAgentEngineWithLLM` or standard).
3. **Parse and Start:**
   - If an argument is provided, use `CampaignParser` to parse the YAML.
   - Start the `AgentEngine` in a background goroutine so it doesn't block the Bubbletea UI thread.
   - Ensure the engine correctly updates the `SessionStore` so the TUI sees live progress via its tick loop.
4. **Makefile Update:**
   - Update the `Makefile` `run` target to accept a campaign argument natively. For example:
     ```make
     run: build
         ./bin/qa-orchestrator $(ARGS)
     ```

## Phase 2: Visual Overhaul & Layout Separation
**Goal:** Create a stunning, organized layout using `lipgloss` with clear separation of concerns.

1. **Grid Layout:**
   - Replace the simplistic vertical/horizontal joins with a structured grid.
   - **Left Panel:** Campaign Info & Flow Status list (always visible, cleanly bordered).
   - **Right Panel:** Active Run Details & Live Trace events (split vertically).
2. **Modern Aesthetics:**
   - Add polished borders (`lipgloss.RoundedBorder()` or `lipgloss.NormalBorder()`) to all panels.
   - Use clear color coding for states (Green=Pass, Red=Fail, Yellow=Paused, Blue/Cyan=Running with animations).
   - Ensure `width` and `height` are dynamically calculated based on terminal size (`tea.WindowSizeMsg`).
3. **Fix the 'f' View:**
   - Instead of hiding the entire screen when `f` is pressed, either make Flow Status a permanent pane or implement a clean modal/overlay system.

## Phase 3: Interactive Navigation & Steering
**Goal:** Make the TUI easy to navigate and responsive to user controls.

1. **Focus Management:**
   - Implement a concept of "active pane" so users know which list they are scrolling (Campaigns vs. Flows vs. Traces).
   - Use `Tab` to switch focus between the panes, highlighting the active pane's border.
2. **Refined Steering Mode:**
   - When the user presses `s` (Steering Mode), render a polished command bar fixed to the bottom of the screen (similar to Vim/Neovim command line), instead of just appending text to the view.
   - Handle cursor blinking and proper text input using Bubbletea's `textinput` component instead of manual rune concatenation.
3. **Dynamic Feedback:**
   - Show a spinner (`charmbracelet/bubbles/spinner`) next to running flows or the active agent to give a sense of life/progress.