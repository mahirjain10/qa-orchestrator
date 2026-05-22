package runtime

import (
	"sync"

	"qa-orchestrator/packages/shared/types"
)

type LifecycleController struct {
	mu         sync.RWMutex
	runID      string
	status     types.RunState
	steeringCh chan *types.SteeringEvent
	cancelCh   chan struct{}
}

func NewLifecycleController(runID string) *LifecycleController {
	return &LifecycleController{
		runID:      runID,
		status:     types.RunStatePending,
		steeringCh: make(chan *types.SteeringEvent, 10),
		cancelCh:   make(chan struct{}, 1),
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
	if !c.CanCancel() {
		return false
	}
	c.mu.Lock()
	c.status = types.RunStateCancelling
	c.mu.Unlock()
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
	select {
	case c.steeringCh <- event:
		return true
	default:
		return false
	}
}

func (c *LifecycleController) DrainSteeringEvents() []*types.SteeringEvent {
	var events []*types.SteeringEvent
	for {
		select {
		case evt := <-c.steeringCh:
			events = append(events, evt)
		default:
			return events
		}
	}
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
