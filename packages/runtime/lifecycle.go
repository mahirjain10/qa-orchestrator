package runtime

import (
	"sync"

	"qa-orchestrator/packages/shared/types"
)

const (
	maxSteeringQueue = 200
)

type LifecycleController struct {
	mu              sync.RWMutex
	runID           string
	status          types.RunState
	steeringMu      sync.Mutex
	steering        []*types.SteeringEvent
	cancelCh        chan struct{}
	overflowDropped int
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
	c.status = status
}

func (c *LifecycleController) Transition(from, to types.RunState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != from {
		return false
	}
	c.status = to
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
	switch c.status {
	case types.RunStateCompleted, types.RunStateCancelled:
		return false
	}
	c.status = types.RunStateCancelling
	select {
	case c.cancelCh <- struct{}{}:
	default:
	}
	return true
}

func (c *LifecycleController) AcknowledgeCancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = types.RunStateCancelled
}

func (c *LifecycleController) CancelCh() <-chan struct{} {
	return c.cancelCh
}

func (c *LifecycleController) SubmitSteering(event *types.SteeringEvent) bool {
	c.steeringMu.Lock()
	if len(c.steering) >= maxSteeringQueue {
		trim := len(c.steering) - maxSteeringQueue + 1
		if trim <= 0 {
			trim = 1
		}
		c.steering = c.steering[trim:]
	}
	c.steering = append(c.steering, event)
	c.steeringMu.Unlock()
	return true
}

func (c *LifecycleController) DrainSteeringEvents() []*types.SteeringEvent {
	c.steeringMu.Lock()
	events := c.steering
	c.steering = make([]*types.SteeringEvent, 0, 64)
	c.steeringMu.Unlock()
	return events
}

func (c *LifecycleController) SetWaitingForInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = types.RunStateWaitingInput
}

func (c *LifecycleController) IsWaitingForInput() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == types.RunStateWaitingInput
}

func (c *LifecycleController) AcknowledgeInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = types.RunStateRunning
}
