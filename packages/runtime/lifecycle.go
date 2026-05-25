package runtime

import (
	"sync"

	"qa-orchestrator/packages/shared/types"
)

const (
	maxSteeringQueue = 200
)

type LifecycleController struct {
	mu             sync.RWMutex
	runID          string
	status         types.RunState
	steeringMu     sync.Mutex
	steering       []*types.SteeringEvent
	cancelCh       chan struct{}
	onStatusChange func(runID string, old, new types.RunState)
}

func (c *LifecycleController) SetOnStatusChange(fn func(runID string, old, new types.RunState)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStatusChange = fn
}

func (c *LifecycleController) setStatusLocked(new types.RunState) {
	old := c.status
	c.status = new
	if c.onStatusChange != nil {
		c.onStatusChange(c.runID, old, new)
	}
}

func NewLifecycleController(runID string) *LifecycleController {
	return &LifecycleController{
		runID:    runID,
		status:   types.RunStatePending,
		steering: make([]*types.SteeringEvent, 0, 64),
		cancelCh: make(chan struct{}, 1),
	}
}

func (c *LifecycleController) GetRunID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runID
}

func (c *LifecycleController) SetRunID(runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runID = runID
}

func (c *LifecycleController) GetStatus() types.RunState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *LifecycleController) SetStatus(status types.RunState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !transitionValid(c.status, status) {
		return
	}
	c.setStatusLocked(status)
}

// BeginExecution atomically transitions PENDING→RUNNING and returns
// the cancel channel for a non-blocking immediate-cancel check.
// Returns ok=false if the transition is invalid.
func (c *LifecycleController) BeginExecution() (cancelCh <-chan struct{}, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !transitionValid(c.status, types.RunStateRunning) {
		return nil, false
	}
	c.setStatusLocked(types.RunStateRunning)
	return c.cancelCh, true
}

func (c *LifecycleController) Transition(from, to types.RunState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != from {
		return false
	}
	if !transitionValid(from, to) {
		return false
	}
	c.setStatusLocked(to)
	return true
}

func (c *LifecycleController) CanCancel() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	switch c.status {
	case types.RunStateCompleted, types.RunStateCancelled:
		return false
	default:
		return true
	}
}

func (c *LifecycleController) RequestCancel() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status == types.RunStateCancelling {
		return true
	}
	if !transitionValid(c.status, types.RunStateCancelling) {
		return false
	}
	c.setStatusLocked(types.RunStateCancelling)
	select {
	case c.cancelCh <- struct{}{}:
	default:
		// Channel full — cancel already pending, don't block
	}
	return true
}

func (c *LifecycleController) AcknowledgeCancel() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status != types.RunStateCancelling {
		return false
	}
	c.setStatusLocked(types.RunStateCancelled)
	return true
}

func (c *LifecycleController) CancelCh() <-chan struct{} {
	return c.cancelCh
}

func (c *LifecycleController) SubmitSteering(event *types.SteeringEvent) bool {
	c.steeringMu.Lock()
	defer c.steeringMu.Unlock()
	if len(c.steering) >= maxSteeringQueue {
		trim := len(c.steering) - maxSteeringQueue + 1
		if trim <= 0 {
			trim = 1
		}
		c.steering = c.steering[trim:]
	}
	c.steering = append(c.steering, event)
	return true
}

func (c *LifecycleController) DrainSteeringEvents() []*types.SteeringEvent {
	c.steeringMu.Lock()
	defer c.steeringMu.Unlock()
	events := c.steering
	c.steering = make([]*types.SteeringEvent, 0, 64)
	return events
}

func (c *LifecycleController) SetWaitingForInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !transitionValid(c.status, types.RunStateWaitingInput) {
		return
	}
	c.setStatusLocked(types.RunStateWaitingInput)
}

func (c *LifecycleController) IsWaitingForInput() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == types.RunStateWaitingInput
}

func (c *LifecycleController) AcknowledgeInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status != types.RunStateWaitingInput {
		return
	}
	c.setStatusLocked(types.RunStateRunning)
}

func transitionValid(from, to types.RunState) bool {
	if from == to {
		return true
	}
	switch from {
	case types.RunStatePending:
		return to == types.RunStateRunning
	case types.RunStateRunning:
		return to == types.RunStatePausing ||
			to == types.RunStateCancelling ||
			to == types.RunStateCompleted ||
			to == types.RunStateFailed ||
			to == types.RunStateWaitingInput
	case types.RunStatePausing:
		return to == types.RunStatePaused
	case types.RunStatePaused:
		return to == types.RunStateResuming ||
			to == types.RunStateCancelling
	case types.RunStateResuming:
		return to == types.RunStateRunning
	case types.RunStateWaitingInput:
		return to == types.RunStateRunning ||
			to == types.RunStateCancelling
	case types.RunStateCancelling:
		return to == types.RunStateCancelled
	case types.RunStateCancelled, types.RunStateCompleted, types.RunStateFailed:
		return false
	}
	return false
}
