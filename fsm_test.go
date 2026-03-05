package appointmentbooking

import (
	"errors"
	"testing"
)

func TestFSM(t *testing.T) {
	// Valid transitions
	validTests := []struct {
		current  string
		event    string
		expected string
	}{
		{StatusPending, EventConfirm, StatusConfirmed},
		{StatusPending, EventCancel, StatusCancelled},
		{StatusPending, EventExpire, StatusExpired},
		{StatusPending, EventReschedule, StatusRescheduled},

		{StatusConfirmed, EventCancel, StatusCancelled},
		{StatusConfirmed, EventComplete, StatusCompleted},
		{StatusConfirmed, EventNoShow, StatusNoShow},
		{StatusConfirmed, EventReschedule, StatusRescheduled},
	}

	for _, tc := range validTests {
		t.Run(tc.current+"_"+tc.event, func(t *testing.T) {
			next, err := Transition(tc.current, tc.event)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if next != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, next)
			}
		})
	}

	// Invalid transitions
	invalidTests := []struct {
		current string
		event   string
	}{
		{StatusPending, EventComplete},
		{StatusPending, EventNoShow},
		{StatusConfirmed, EventConfirm},
		{StatusConfirmed, EventExpire},
	}

	for _, tc := range invalidTests {
		t.Run("invalid_"+tc.current+"_"+tc.event, func(t *testing.T) {
			_, err := Transition(tc.current, tc.event)
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected ErrInvalidTransition, got %v", err)
			}
		})
	}

	// Terminal states
	terminalStates := []string{
		StatusCancelled,
		StatusCompleted,
		StatusNoShow,
		StatusExpired,
		StatusRescheduled,
	}

	for _, state := range terminalStates {
		t.Run("terminal_"+state, func(t *testing.T) {
			if !IsTerminal(state) {
				t.Fatalf("expected %s to be terminal", state)
			}

			// Try to apply any event to a terminal state
			_, err := Transition(state, EventConfirm)
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected ErrInvalidTransition for terminal state %s, got %v", state, err)
			}
		})
	}

	// Non-terminal states
	nonTerminalStates := []string{
		StatusPending,
		StatusConfirmed,
	}

	for _, state := range nonTerminalStates {
		t.Run("non_terminal_"+state, func(t *testing.T) {
			if IsTerminal(state) {
				t.Fatalf("expected %s to be non-terminal", state)
			}
		})
	}

	// Invalid state entirely
	t.Run("invalid_state", func(t *testing.T) {
		_, err := Transition("UNKNOWN_STATE", EventConfirm)
		if !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("expected ErrInvalidTransition for unknown state, got %v", err)
		}
	})
}
