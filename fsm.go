package appointmentbooking

import "errors"

// States
const (
	StatusPending     = "PENDING"
	StatusConfirmed   = "CONFIRMED"
	StatusCancelled   = "CANCELLED"
	StatusCompleted   = "COMPLETED"
	StatusNoShow      = "NO_SHOW"
	StatusExpired     = "EXPIRED"     // Unpaid reservation that timed out (trigger: external scheduler via MCP)
	StatusRescheduled = "RESCHEDULED" // Original reservation superseded by a new one (audit trail)
)

// Events
const (
	EventConfirm    = "CONFIRM"
	EventCancel     = "CANCEL"
	EventComplete   = "COMPLETE"
	EventNoShow     = "NO_SHOW_EVENT"
	EventExpire     = "EXPIRE"
	EventReschedule = "RESCHEDULE" // Marks original as RESCHEDULED; new reservation created atomically
)

// transitions[currentState][event] = nextState
var transitions = map[string]map[string]string{
	StatusPending: {
		EventConfirm:    StatusConfirmed,
		EventCancel:     StatusCancelled,
		EventExpire:     StatusExpired,
		EventReschedule: StatusRescheduled,
	},
	StatusConfirmed: {
		EventCancel:     StatusCancelled,
		EventComplete:   StatusCompleted,
		EventNoShow:     StatusNoShow,
		EventReschedule: StatusRescheduled,
	},
	// CANCELLED, COMPLETED, NO_SHOW, EXPIRED, RESCHEDULED are terminal — no outgoing transitions
}

// ErrInvalidTransition is returned when a transition is not allowed.
var ErrInvalidTransition = errors.New("invalid transition")

// Transition returns the next state or an error if the transition is invalid.
func Transition(current, event string) (string, error) {
	if IsTerminal(current) {
		return "", ErrInvalidTransition
	}
	nextState, ok := transitions[current][event]
	if !ok {
		return "", ErrInvalidTransition
	}
	return nextState, nil
}

// IsTerminal returns true if the status has no outgoing transitions.
func IsTerminal(status string) bool {
	switch status {
	case StatusCancelled, StatusCompleted, StatusNoShow, StatusExpired, StatusRescheduled:
		return true
	default:
		return false
	}
}
