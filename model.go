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

// ormc:formonly
type TimeSlot struct {
	StartUTC int64
	EndUTC   int64
}

// ormc:formonly
type createReservationArgs struct {
	TenantID                string
	ClientID                string
	CreatorUserID           string
	EmployeeServiceConfigID string
	SlotStartUTC            int64
	Notes                   string
	RescheduledFromID       string
}

// ormc:formonly
type getReservationArgs struct {
	TenantID string
	ID       string
}

// ormc:formonly
type listReservationsByStaffArgs struct {
	TenantID string
	StaffID  string
	From     int64
	To       int64
}

// ormc:formonly
type listReservationsByClientArgs struct {
	TenantID string
	ClientID string
}

// ormc:formonly
type changeReservationStatusArgs struct {
	TenantID  string
	ID        string
	Event     string
	ActorID   string
	PaymentID string
	Revision  int64
}

// ormc:formonly
type expirePendingReservationsArgs struct {
	TenantID string
	Before   int64
}

// ormc:formonly
type upsertCalendarConfigArgs struct {
	TenantID string
	StaffID  string
	Timezone string
	IsActive bool
}

// ormc:formonly
type upsertWeeklyCalendarArgs struct {
	TenantID    string
	StaffID     string
	DayOfWeek   int64
	WorkStart   int64
	WorkFinish  int64
	BreakStart  int64
	BreakFinish int64
	IsActive    bool
}

// ormc:formonly
type addCalendarExceptionArgs struct {
	TenantID      string
	StaffID       string
	SpecificDate  int64
	ExceptionType string
	StartTime     int64
	EndTime       int64
	Notes         string
}

// ormc:formonly
type removeCalendarExceptionArgs struct {
	TenantID    string
	ExceptionID string
}

// ormc:formonly
type listAvailabilityArgs struct {
	TenantID string
	StaffID  string
	ConfigID string
	From     int64
	To       int64
}
