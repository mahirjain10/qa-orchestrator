package shared

import "errors"

var (
	ErrNotFound               = errors.New("not found")
	ErrModelRequired          = errors.New("model is required")
	ErrAlreadyRunning         = errors.New("already running")
	ErrNotRunning             = errors.New("not running")
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrFlowNotFound           = errors.New("flow not found")
	ErrNoRunSelected          = errors.New("no run selected")
	ErrCancelled              = errors.New("cancelled before execution")
)
