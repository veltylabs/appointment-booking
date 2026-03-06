//go:build !wasm

package appointmentbooking

// EmployeeServiceConfig maps per-staff-per-service durations and overrides.
type EmployeeServiceConfig struct {
	ID              string `db:"pk"`
	TenantID        string
	StaffID         string
	ServiceID       string
	DurationMin     int64
	BufferMin       int64
	PriceOverride   float64
	PaymentRequired bool
	IsActive        bool
}

// WorkCalendarConfig is the single source of truth for timezone per staff.
type WorkCalendarConfig struct {
	ID       string `db:"pk"`
	TenantID string
	StaffID  string
	Timezone string // IANA e.g. "America/Santiago"
	IsActive bool
}

// WorkCalendarWeekly defines recurring weekly hours for a staff member.
type WorkCalendarWeekly struct {
	ID          string `db:"pk"`
	TenantID    string
	StaffID     string
	DayOfWeek   int64 // 0=Sunday … 6=Saturday
	WorkStart   int64 // minutes from midnight, local time
	WorkFinish  int64
	BreakStart  int64
	BreakFinish int64
	IsActive    bool
}

// WorkCalendarException overrides working hours for a specific date.
type WorkCalendarException struct {
	ID            string `db:"pk"`
	TenantID      string
	StaffID       string
	SpecificDate  int64  // unix timestamp (UTC midnight)
	ExceptionType string // "day_off" | "custom_hours"
	StartTime     int64
	EndTime       int64
	Notes         string
}

// Reservation is the core booking record.
type Reservation struct {
	ID                      string `db:"pk"`
	TenantID                string
	ClientID                string
	CreatorUserID           string
	EmployeeServiceConfigID string
	StaffIDSnapshot         string
	ServiceIDSnapshot       string
	DurationMinSnapshot     int64
	PriceSnapshot           float64
	CurrencySnapshot        string
	ReservationDate         int64  // unix timestamp of the LOCAL date (UTC midnight)
	ReservationTime         int64  // unix timestamp (UTC)
	LocalStringDate         string // "2026-03-04"
	LocalStringTime         string // "14:30"
	Status                  string // FSM state — use constants from fsm.go (StatusPending, StatusConfirmed, etc.)
	RescheduledFromID       string
	PaymentID               string
	Notes                   string
	UpdatedAt               int64
	UpdatedBy               string
	Revision                int64
}

// TimeSlot is returned by ListAvailability
type TimeSlot struct {
	StartUTC int64 `db:"-"`
	EndUTC   int64 `db:"-"`
}
