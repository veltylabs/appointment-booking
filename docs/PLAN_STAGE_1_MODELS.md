# Stage 1 — Models + FSM

← Master: [PLAN.md](PLAN.md) | Next → [Stage 2 — ORM](PLAN_STAGE_2_ORM.md)

## Goal

Define all Go structs and the reservation FSM. No DB or service logic yet.

---

## Files to create

| File | Package | Purpose |
|---|---|---|
| `model.go` | `appointmentbooking` | All entity structs |
| `fsm.go` | `appointmentbooking` | FSM states, events, transition map |
| `fsm_test.go` | `appointmentbooking` | Unit tests for FSM transitions |

---

## 1. model.go — Structs

```go
type EmployeeServiceConfig struct {
    ID             string
    TenantID       string
    StaffID        string
    ServiceID      string
    DurationMin    int
    BufferMin      int     // prep/cleaning time between slots
    PriceOverride  float64
    PaymentRequired bool
    IsActive       bool
}

type Reservation struct {
    ID                      string
    TenantID                string
    ClientID                string
    CreatorUserID           string
    EmployeeServiceConfigID string
    StaffIDSnapshot         string
    ServiceIDSnapshot       string
    DurationMinSnapshot     int
    PriceSnapshot           float64 // financial auditability
    CurrencySnapshot        string  // default "CLP" or tenant currency
    ReservationDate         int64   // Unix UTC — date only (midnight)
    ReservationTime         int64   // Unix UTC — exact moment
    LocalStringDate         string  // "YYYY-MM-DD" for easier analytics
    LocalStringTime         string  // "HH:MM" for easier analytics
    Status                  string  // FSM state
    RescheduledFromID       string  // optional: previous reservation ID
    PaymentID               string  // soft ref to payment module (nullable)
    Notes                   string
    UpdatedAt               int64
    UpdatedBy               string
    Revision                int
}

// WorkCalendarConfig — one row per staff. Single source of truth for IANA timezone.
// WorkCalendarWeekly and WorkCalendarException inherit timezone from this entity.
type WorkCalendarConfig struct {
    ID       string
    TenantID string
    StaffID  string
    Timezone string // IANA, e.g. "America/Santiago"
    IsActive bool
}

type WorkCalendarWeekly struct {
    ID          string
    TenantID    string
    StaffID     string
    // No Timezone here — loaded from WorkCalendarConfig
    DayOfWeek   int  // 0=Sunday … 6=Saturday
    WorkStart   int  // local time integer, e.g. 900 = 09:00
    WorkFinish  int
    BreakStart  int
    BreakFinish int
    IsActive    bool
}

type WorkCalendarException struct {
    ID            string
    TenantID      string
    StaffID       string
    SpecificDate  int64  // Unix UTC midnight
    ExceptionType string // "HOLIDAY" | "SPECIAL_HOURS" | "BLOCKED"
    StartTime     int    // local time integer (interpreted using timezone from WorkCalendarConfig)
    EndTime       int
    Notes         string
}

// TimeSlot is returned by ListAvailability
type TimeSlot struct {
    StartUTC int64
    EndUTC   int64
}
```

---

## 2. fsm.go — FSM

### States (constants)

```go
const (
    StatusPending      = "PENDING"
    StatusConfirmed    = "CONFIRMED"
    StatusCancelled    = "CANCELLED"
    StatusCompleted    = "COMPLETED"
    StatusNoShow       = "NO_SHOW"
    StatusExpired      = "EXPIRED"      // Unpaid reservation that timed out (trigger: external scheduler via MCP)
    StatusRescheduled  = "RESCHEDULED"  // Original reservation superseded by a new one (audit trail)
)
```

### Events (constants)

```go
const (
    EventConfirm    = "CONFIRM"
    EventCancel     = "CANCEL"
    EventComplete   = "COMPLETE"
    EventNoShow     = "NO_SHOW_EVENT"
    EventExpire     = "EXPIRE"
    EventReschedule = "RESCHEDULE" // Marks original as RESCHEDULED; new reservation created atomically
)
```

### Transition map

```go
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

// Transition returns the next state or an error if the transition is invalid.
func Transition(current, event string) (string, error) { ... }

// IsTerminal returns true if the status has no outgoing transitions.
func IsTerminal(status string) bool { ... }
```

---

## 3. fsm_test.go — Tests

Cover all branches:
- Valid transitions for each state+event pair
- Invalid transitions return error
- Terminal states reject all events
- `IsTerminal` returns true for CANCELLED, COMPLETED, NO_SHOW, EXPIRED, RESCHEDULED

## 4. docs/diagrams/fsm.md

Already created — see [FSM Diagram](diagrams/fsm.md). No action required.

---

## Acceptance criteria

- [ ] All structs compile with `go build`
- [ ] `gotest -run TestFSM` passes with 100% branch coverage of `fsm.go`
- [ ] No external dependencies added
