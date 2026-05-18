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
	pauseCh    chan struct{}
	resumeCh   chan struct{}
	cancelCh   chan struct{}
	inputCh    chan struct{}
}

func NewLifecycleController(runID string) *LifecycleController {
	return &LifecycleController{
		runID:      runID,
		status:     types.RunStatePending,
		steeringCh: make(chan *types.SteeringEvent, 10),
		pauseCh:    make(chan struct{}, 1),
		resumeCh:   make(chan struct{}, 1),
		cancelCh:   make(chan struct{}, 1),
		inputCh:    make(chan struct{}, 1),
	}
}

func (c *LifecycleController) GetRunID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runID
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

func (c *LifecycleController) CanPause() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == types.RunStateRunning || c.status == types.RunStatePending
}

func (c *LifecycleController) CanResume() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == types.RunStatePaused
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

func (c *LifecycleController) RequestPause() bool {
	if !c.CanPause() {
		return false
	}
	c.mu.Lock()
	c.status = types.RunStatePausing
	c.mu.Unlock()
	select {
	case c.pauseCh <- struct{}{}:
	default:
	}
	return true
}

func (c *LifecycleController) AcknowledgePause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = types.RunStatePaused
}

func (c *LifecycleController) RequestResume() bool {
	if !c.CanResume() {
		return false
	}
	c.mu.Lock()
	c.status = types.RunStateResuming
	c.mu.Unlock()
	select {
	case c.resumeCh <- struct{}{}:
	default:
	}
	return true
}

func (c *LifecycleController) AcknowledgeResume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = types.RunStateRunning
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

func (c *LifecycleController) WaitForPause() {
	<-c.pauseCh
}

func (c *LifecycleController) WaitForResume() {
	<-c.resumeCh
}

func (c *LifecycleController) WaitForCancel() {
	<-c.cancelCh
}

func (c *LifecycleController) PauseCh() <-chan struct{} {
	return c.pauseCh
}

func (c *LifecycleController) ResumeCh() <-chan struct{} {
	return c.resumeCh
}

func (c *LifecycleController) CancelCh() <-chan struct{} {
	return c.cancelCh
}

func (c *LifecycleController) SubmitSteering(event *types.SteeringEvent) {
	select {
	case c.steeringCh <- event:
	default:
	}
}

func (c *LifecycleController) SteerCh() <-chan *types.SteeringEvent {
	return c.steeringCh
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

func (c *LifecycleController) RequestInput() {
	c.SetWaitingForInput()
	select {
	case c.inputCh <- struct{}{}:
	default:
	}
}

func (c *LifecycleController) InputCh() <-chan struct{} {
	return c.inputCh
}

func (c *LifecycleController) SetCompleted() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = types.RunStateCompleted
}

func (c *LifecycleController) SetFailed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = types.RunStateFailed
}
