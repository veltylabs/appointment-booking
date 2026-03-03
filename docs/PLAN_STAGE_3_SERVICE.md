# Stage 3 — Service Layer

← [Stage 2 — ORM](PLAN_STAGE_2_ORM.md) | Next → [Stage 4 — MCP](PLAN_STAGE_4_MCP.md)

## Goal

Implement the `SchedulingService` interface with full availability and reservation logic.
All external module dependencies injected via interfaces.

---

## Files to create

| File | Purpose |
|---|---|
| `service.go` | `SchedulingService` interface + implementation struct (holds `*orm.DB`) |
| `service_front_test.go` | WASM tests — pure FSM/algorithm logic only (`//go:build wasm`) |
| `service_back_test.go` | Backend tests — real `tinywasm/sqlite` in-memory DB (`//go:build !wasm`) |
| `setup_test.go` | Shared test runner and setup |

---

## 1. Service struct — direct DB access

The service holds `*orm.DB` directly. There are **no** `ReservationStore`, `CalendarStore`, or `ConfigStore` interfaces. All DB operations call the ORM functions defined in `model_orm.go` (Stage 2) directly. This allows tests to exercise the real ORM + SQLite stack with an in-memory database, catching potential bugs in the full stack rather than hiding them behind mocks.

```go
type schedulingService struct {
    db        *orm.DB
    staff     StaffReader
    catalog   CatalogReader
    directory DirectoryReader
    pub       EventPublisher // nil = events disabled
}

func New(db *orm.DB, deps Deps) SchedulingService {
    return &schedulingService{
        db:        db,
        staff:     deps.Staff,
        catalog:   deps.Catalog,
        directory: deps.Directory,
        pub:       deps.Publisher,
    }
}

type Deps struct {
    Staff     StaffReader
    Catalog   CatalogReader
    Directory DirectoryReader
    Publisher EventPublisher // nil = events disabled
}
```

---

## 2. External dependency interfaces (defined in service.go)

```go
// StaffReader verifies a staff member exists and belongs to the tenant.
type StaffReader interface {
    StaffExists(tenantID, staffID string) (bool, error)
}

// CatalogReader verifies a service exists and belongs to the tenant.
type CatalogReader interface {
    ServiceExists(tenantID, serviceID string) (bool, error)
}

// DirectoryReader verifies a client exists and belongs to the tenant.
type DirectoryReader interface {
    ClientExists(tenantID, clientID string) (bool, error)
}

// EventPublisher delivers domain events to other modules or infrastructure.
// Pass nil to disable event broadcasting safely (e.g., in tests or CLI tools).
type EventPublisher interface {
    Publish(ctx context.Context, event string, payload any) error
}

// Domain events emitted by this module (event name constants).
const (
    EventReservationCreated     = "appointment.reservation.created"
    EventReservationConfirmed   = "appointment.reservation.confirmed"
    EventReservationCancelled   = "appointment.reservation.cancelled"
    EventReservationCompleted   = "appointment.reservation.completed"
    EventReservationNoShow      = "appointment.reservation.no_show"
    EventReservationExpired     = "appointment.reservation.expired"
    EventReservationRescheduled = "appointment.reservation.rescheduled" // emitted for the original reservation
)
```

These interfaces are injected into the service — never imported directly.

**Identity contract:** `actorID` (present in `CreateReservationCmd`, `ChangeStatusCmd`) is a plain string
received from the MCP caller. This module NEVER validates roles or permissions — that is the
responsibility of the gateway/middleware that calls the MCP tool. The actorID is stored as an
audit field only.

---

## 3. SchedulingService interface

```go
type SchedulingService interface {
    // Calendar management
    UpsertCalendarConfig(ctx context.Context, cfg WorkCalendarConfig) error
    UpsertWeeklyCalendar(ctx context.Context, cal WorkCalendarWeekly) error
    AddException(ctx context.Context, exc WorkCalendarException) error
    RemoveException(ctx context.Context, tenantID, exceptionID string) error

    // Availability
    ListAvailability(ctx context.Context, tenantID, staffID, configID string, from, to int64) ([]TimeSlot, error)

    // Reservations
    CreateReservation(ctx context.Context, cmd CreateReservationCmd) (Reservation, error)
    GetReservation(ctx context.Context, tenantID, id string) (Reservation, error)
    ListReservationsByStaff(ctx context.Context, tenantID, staffID string, from, to int64) ([]Reservation, error)
    ListReservationsByClient(ctx context.Context, tenantID, clientID string) ([]Reservation, error)
    ChangeReservationStatus(ctx context.Context, cmd ChangeStatusCmd) error
    ExpirePendingReservations(ctx context.Context, tenantID string, before int64) error
}

type CreateReservationCmd struct {
    TenantID                string
    ClientID                string
    CreatorUserID           string
    EmployeeServiceConfigID string
    SlotStartUTC            int64
    Notes                   string
    RescheduledFromID       string // optional
}

type ChangeStatusCmd struct {
    TenantID  string
    ID        string
    Event     string // FSM event constant
    ActorID   string // who triggers the change
    PaymentID string // optional, only applied when Event == EventConfirm
    Revision  int
}
```

---

## 4. ListAvailability algorithm

### Why local integers + IANA timezone (not pure UTC)?

`work_start`/`work_finish` are stored as local integers (e.g., `900 = 09:00`) combined with an IANA timezone string. This is intentional:

- A recurring rule "work 09:00–18:00 every Monday" must stay at 09:00 local time even after Daylight Saving Time (DST) transitions. Storing absolute Unix timestamps for recurring weekly rules would silently shift the schedule by 1 hour on DST boundary dates.
- The conversion `(local_integer, timezone, date) → Unix UTC` must be done at query time using the `tinywasm/time` package.

### Mandatory conversion step

Before generating slots for any day, convert local work boundaries to UTC:

```
workStartUTC = LocalIntToUnixUTC(day, WorkStart, Timezone)
workFinishUTC = LocalIntToUnixUTC(day, WorkFinish, Timezone)
```

Where `LocalIntToUnixUTC(date int64, localInt int, tz string) int64` interprets `localInt`
as `HH*100+MM` on the given `date` in the given `tz`.

### Algorithm

```
1. Load WorkCalendarConfig for (tenantID, staffID) — must exist and be active. Extract Timezone.
2. Load WorkCalendarWeekly for (tenantID, staffID) — active rows only.
3. Load WorkCalendarException for (tenantID, staffID) within [from, to].
4. Load existing Reservations for (tenantID, staffID) within [from, to] — non-CANCELLED, non-RESCHEDULED, non-EXPIRED.
5. Load EmployeeServiceConfig by configID — must be active, belong to tenantID.
6. For each day D in [from, to]:
   a. Find the weekly rule for D's day-of-week. If none → skip day (no working hours).
   b. Convert work boundaries using Timezone from WorkCalendarConfig:
      workStartUTC  = LocalIntToUnixUTC(D, weekly.WorkStart,  Timezone)
      workFinishUTC = LocalIntToUnixUTC(D, weekly.WorkFinish, Timezone)
      breakStartUTC = LocalIntToUnixUTC(D, weekly.BreakStart, Timezone)
      breakFinishUTC= LocalIntToUnixUTC(D, weekly.BreakFinish,Timezone)
   c. Apply exceptions for day D (in priority order):
      - HOLIDAY → no slots for this day; continue to next day.
      - SPECIAL_HOURS → replace workStartUTC/workFinishUTC with
        LocalIntToUnixUTC(D, exc.StartTime, Timezone) / LocalIntToUnixUTC(D, exc.EndTime, Timezone);
        break interval is removed.
      - BLOCKED → subtract [LocalIntToUnixUTC(D, exc.StartTime, Timezone),
                              LocalIntToUnixUTC(D, exc.EndTime,   Timezone)] from available window.
      - If multiple exceptions exist on the same day: HOLIDAY > SPECIAL_HOURS > BLOCKED.
   d. Generate slots of `duration_min` within the available window, skipping the break interval.
      - A slot [s, s+duration] is only generated if it fits entirely before breakStartUTC or starts at/after breakFinishUTC.
      - A slot is only generated if s+duration+buffer_min ≤ workFinishUTC (buffer is internal clinic time, not exposed in TimeSlot).
      - Returned TimeSlot: { StartUTC: s, EndUTC: s + duration_min*60 } — buffer excluded from client-visible window.
   e. Remove slots that overlap with existing reservations.
      An existing reservation of duration D_r blocks the interval [res.ReservationTime, res.ReservationTime + (D_r + buffer_r)*60].
7. Return all free TimeSlots.
```

---

## 5. UpsertWeeklyCalendar — requires WorkCalendarConfig

`WorkCalendarWeekly` has no timezone field. Before upserting weekly rows, the staff calendar config MUST already exist. The service enforces this:

```
1. Load WorkCalendarConfig for (tenantID, staffID).
   If not found → return ErrCalendarConfigNotFound.
   (Caller must first call UpsertCalendarConfig to set the timezone.)
2. Proceed with upsert of WorkCalendarWeekly.
```

`UpsertCalendarConfig` is a separate service operation that creates or updates the `WorkCalendarConfig` for a staff member. It can update the timezone freely (no conflict possible — there is only one config row per staff).

`ErrCalendarConfigNotFound` is a sentinel error defined in this package. The MCP handler for `upsert_weekly_calendar` must surface it as a descriptive, actionable message (e.g., "Set the staff timezone first using upsert_calendar_config").

---

## 6. CreateReservation logic

```
1. Load EmployeeServiceConfig by (tenantID, configID) — must exist and be active.
2. Validate clientID via DirectoryReader.ClientExists(tenantID, clientID).
3. Validate staffID via StaffReader.StaffExists(tenantID, config.StaffID).
4. Validate serviceID via CatalogReader.ServiceExists(tenantID, config.ServiceID).
5. Call ListAvailability for the target day — verify SlotStartUTC is among the returned free slots.
6. *Database Transaction Begins*
7. Build Reservation:
   - Status = StatusPending
   - Snapshots: PriceSnapshot, CurrencySnapshot, DurationMinSnapshot, ServiceIDSnapshot, StaffIDSnapshot from config
   - LocalStringDate / LocalStringTime derived from SlotStartUTC + staff timezone
   - RescheduledFromID if provided
8. If RescheduledFromID is set, within the SAME transaction:
   a. Load original reservation via RES.GetReservation(tenantID, rescheduledFromID).
   b. Call FSM.Transition(original.Status, EventReschedule) — error if invalid.
   c. Call RES.UpdateReservationStatus(id, RESCHEDULED, actorID, now, revision).
   Note: do NOT route through the public ChangeReservationStatus() method — it would open
   a separate operation outside the current transaction context.
9. Insert Reservation. If a duplicate slot was concurrently booked (unique constraint violation) → rollback and return ErrSlotTaken.
10. *Database Transaction Commits*.
11. If pub != nil: pub.Publish(ctx, EventReservationCreated, newReservation).
    If RescheduledFromID was set: also pub.Publish(ctx, EventReservationRescheduled, originalReservation).
    Publish errors are logged but do NOT fail the operation (fire-and-forget).
12. Return created Reservation.
```

---

## 7. ChangeReservationStatus logic

```
1. Load current Reservation — must belong to tenantID.
2. Call FSM.Transition(current.Status, cmd.Event) — error if invalid.
3. If cmd.Event == EventConfirm && cmd.PaymentID != "": set reservation.PaymentID.
   PaymentID is ignored for all other events.
4. Update via UpdateReservationStatus with optimistic concurrency (revision check).
   On ErrConflict: propagate to caller — do not retry internally.
5. Map cmd.Event to domain event constant:
   CONFIRM       → EventReservationConfirmed
   CANCEL        → EventReservationCancelled
   COMPLETE      → EventReservationCompleted
   NO_SHOW_EVENT → EventReservationNoShow
   EXPIRE        → EventReservationExpired
6. If pub != nil: pub.Publish(ctx, domainEvent, updatedReservation).
   Publish errors are logged but do NOT fail the operation (fire-and-forget).
7. Return nil on success.
```

---

## 8. ExpirePendingReservations logic

Called by an external scheduler (via MCP tool `expire_pending_reservations`). Internally:

```
1. List all PENDING reservations for tenantID where ReservationTime < before.
2. For each: call FSM.Transition(StatusPending, EventExpire) and update status.
3. Return count of expired reservations or first error encountered.
```

---

## 9. Testing (Dual Testing Pattern)

Since `service.go` contains pure logic (`ListAvailability`) that might be executed directly in WASM (frontend offline mode or optimistic UI) or in the Backend, implement the **WASM/Stlib Dual Testing Pattern**.

- `service_front_test.go` (`//go:build wasm`) — tests pure FSM logic and the `ListAvailability` algorithm. Uses mock implementations only for `StaffReader`, `CatalogReader`, `DirectoryReader`, and `EventPublisher` (the four external-module interfaces). DB operations are exercised only in the backend test.
- `service_back_test.go` (`//go:build !wasm`) — full integration tests. Uses a real `tinywasm/sqlite` in-memory database (same setup as Stage 2's `model_orm_back_test.go`). Runs auto-migration at test startup so the real ORM + SQLite stack is exercised end-to-end. Mocks only the three external-module interfaces and `EventPublisher`.
- Both files MUST call a shared test runner `RunServiceTests(t)` defined in `setup_test.go`.

**Why real SQLite in backend tests (not mocked stores)?**
`tinywasm/sqlite` is already configured with `tinywasm/orm` by default. Using a real in-memory database catches real bugs in ORM query generation, constraint enforcement (e.g., unique slot constraint), and optimistic concurrency — scenarios that mocked stores silently paper over.

Test cases:
- `ListAvailability` — weekly only, with HOLIDAY exception, with SPECIAL_HOURS, with BLOCKED interval, slots blocked by existing reservation, break boundary edge case (slot touching break_start must be excluded), DST/timezone conversion
- `UpsertCalendarConfig` — create new, update existing timezone
- `UpsertWeeklyCalendar` — happy path, ErrCalendarConfigNotFound when no config exists for staff
- `CreateReservation` — happy path (EventReservationCreated published), slot not available, invalid client, invalid staff, invalid service, reschedule flow (EventReservationCreated + EventReservationRescheduled published), nil publisher does not panic
- `ChangeReservationStatus` — all valid transitions + correct domain event published, all invalid transitions, concurrent revision conflict (ErrConflict propagated), PaymentID only applied on CONFIRM
- `ExpirePendingReservations` — expires only PENDING before threshold, ignores CONFIRMED, EventReservationExpired published per reservation

---

## Acceptance criteria

- [ ] `gotest` passes on both WASM and standard (`RunServiceTests`).
- [ ] No imports from other velty_modules (only interfaces: `StaffReader`, `CatalogReader`, `DirectoryReader`, `EventPublisher`).
- [ ] Backend tests use real `tinywasm/sqlite` in-memory DB — no store mocks.
- [ ] `ListAvailability` respects break times, `buffer_min`, and timezone conversion.
- [ ] FSM rejects invalid transitions.
- [ ] `ErrCalendarConfigNotFound` returned when `UpsertWeeklyCalendar` is called without prior config.
- [ ] Optimistic concurrency and Reschedule Atomic Transactions tested without race conditions.
- [ ] Original reservation ends as RESCHEDULED (not CANCELLED) on reschedule.
- [ ] Correct domain event published on every FSM transition.
- [ ] nil EventPublisher does not panic — event publishing is fire-and-forget.
