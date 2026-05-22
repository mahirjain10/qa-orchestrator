# TUI & OpenRouter Hardening Phases (V3)

These phases address OpenRouter integration robustness and TUI pane management flexibility.

## Phase 1: OpenRouter Integration Hardening
**Goal:** Ensure robust communication with OpenRouter across diverse models and fix API inconsistencies.

1. **`packages/llm/types.go` Cleanup:**
   - Remove duplicate `Message` struct definition.
   - Update `GenerateResponse` to include `ID`, `Object`, and `Created` fields.
   - Update `Choice` to include `FinishReason` (OpenAI standard).
   - Ensure `Usage` fields are properly tagged for JSON.

2. **`packages/llm/client.go` Robustness:**
   - Update `doRequest` to correctly extract `FinishReason` from `Choices[0]` if top-level `FinishReason` is empty.
   - Improve `parseErrorResponse` to handle nested OpenRouter errors (e.g., when a provider fails but OpenRouter returns a 200 with an error object, or a 4xx with provider-specific metadata).
   - Add support for `Stop` sequences and `TopP` in `GenerateRequest`.
   - Log the `Model` actually used in the response (OpenRouter sometimes falls back or routes differently).

3. **`packages/llm/config.go` Enhancements:**
   - Add optional `LLM_PROVIDER_PRIORITY` or `LLM_PROVIDER_ALLOW` environment variables if needed for OpenRouter routing.

## Phase 2: TUI Rendering Fixes
**Goal:** Stabilize live TUI rendering while campaigns execute.

1. **Flow Status ANSI Rendering:**
   - Render fixed-width status text before applying Lipgloss styling so ANSI escape sequences never enter format strings.

2. **Auto-refresh Polling:**
   - Requeue `tea.Tick` after each `TickMsg` so live polling continues after the first refresh.

## Phase 3: TUI Slot-based Layout Engine
**Goal:** Move from a hardcoded 4-pane view to a flexible "Slot" system that allows any component in any quadrant.

1. **`apps/tui/internal/screens/main.go` Refactoring:**
   - Define `ComponentID` type and constants: `CompCampaigns`, `CompFlows`, `CompRun`, `CompTraces`, `CompArtifacts`, `CompReport`.
   - Add `quadrants [4]ComponentID` to `MainScreen` struct.
   - Initialize defaults: `Q0=CompCampaigns`, `Q1=CompFlows`, `Q2=CompRun`, `Q3=CompTraces`.
   - Add `activeSlot int` (0-3) to track focus.

2. **Render Logic Update:**
   - Rewrite `View()` to iterate through `quadrants` and render the corresponding component in the calculated space.
   - Use a helper `renderComponent(id ComponentID, width, height, focused bool) string`.

3. **Fullscreen/Maximize Mode:**
   - Add `maximized bool` and `maximizedSlot int` to `MainScreen`.
   - When `maximized` is true, `View()` renders only the component in `maximizedSlot` across the full screen or full right column.

## Phase 4: Interactive Pane Management
**Goal:** Allow users to swap and manage panes dynamically.

1. **Keyboard Handlers:**
   - `TAB`: Move `activeSlot` (0 -> 1 -> 2 -> 3 -> 0).
   - `p`: Cycle `ComponentID` for the current `activeSlot`.
   - `m`: Toggle `maximized` for the current `activeSlot`.
   - `ESC`: If `maximized`, set `maximized = false`. If `steeringMode`, exit steering.

2. **Navigation Routing:**
   - Update `Update()` to route `up/down/j/k` messages to the component currently occupying the `activeSlot`.
   - For example, if `activeSlot` is 2 and it contains `CompTraces`, directional keys should scroll the traces.

3. **Visual Feedback:**
   - Ensure the focused quadrant has a distinct border color (already partially implemented, but needs to sync with `activeSlot`).
   - Add a small label in the border of each quadrant indicating which component it is (e.g., " [Traces] ").
## Phase 5: Validation & Testing
1. **OpenRouter Test:**
   - Create a test campaign that uses a specific non-OpenAI model via OpenRouter.
   - Verify usage tracking and finish reason handling.
2. **TUI Layout Test:**
   - Manually verify that swapping panes works and doesn't "lose" previous states.
   - Verify that maximizing a pane and then restoring it returns the exact 4-quadrant state as before.
