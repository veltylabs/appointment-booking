# Test Coverage Plan — appointment-booking

## Development Rules

- **Standard Library Only**: No external assertion libs (`testify`, etc.). Use only `testing` and `errors`.
- **Mocking**: All external interfaces (`StaffReader`, `CatalogReader`, `DirectoryReader`, `EventPublisher`) MUST be mocked.
- **WASM Dual Testing Pattern**: Pure logic tests go in `setup_test.go` (via shared runner). SQLite-specific/integration tests have `//go:build !wasm` tag.
- **Max 500 lines per file**: If `setup_test.go` exceeds limit after additions, split by domain (e.g., `availability_test_runner.go`, `service_test_runner.go`).
- **Test Runner**: Always use `gotest` (not `go test`).
- **Test Location**: All tests reside in `tests/`.

---

## Background

The module implements a scheduling/booking domain. The core flow is:

1. Staff calendars are configured with weekly schedules and exceptions.
2. Clients can query available time slots.
3. Clients create reservations; status transitions are managed by an FSM.
4. Pending reservations can be bulk-expired by an external scheduler.

**Diagrams:**
- [FSM Diagram](diagrams/fsm.md) — All reservation status transitions
- [Test Coverage Mindmap](diagrams/service_tests.md) — Detailed flowcharts per Use Case (UC-01 to UC-20)

---

## Currently Covered Tests

| File | Tests |
|---|---|
| `tests/fsm_test.go` | All FSM valid/invalid transitions, terminal/non-terminal states |
| `tests/setup_test.go` | `CreateReservation_Success`, `CreateReservation_SlotTaken`, `ExpirePendingReservations` |
| `tests/service_back_test.go` | Optimistic-lock conflict (`ErrConflict`), event publisher calls |
| `tests/repository_test.go` | CRUD for all repo entities, upsert idempotency, conflict detection |

---

## Missing Tests (Priority Order)

### Stage 1 — Service Validation Errors

**File**: add cases to `setup_test.go` inside `RunServicePureTests`  
**Goal**: Validate that `CreateReservation` correctly rejects invalid inputs.

#### UC-01: `CreateReservation_InactiveConfig`
- Insert an `EmployeeServiceConfig` with `IsActive: false`.
- Call `CreateReservation` with its ID.
- Expect `ErrNotFound`.

#### UC-02: `CreateReservation_StaffNotFound`
- Override `MockStaffReader.Exists = false`.
- Call `CreateReservation`.
- Expect error with message containing `"staff not found"`.

#### UC-03: `CreateReservation_ServiceNotFound`
- Override `MockCatalogReader.Exists = false`.
- Call `CreateReservation`.
- Expect error with message containing `"service not found"`.

#### UC-04: `CreateReservation_ClientNotFound`
- Override `MockDirectoryReader.Exists = false`.
- Call `CreateReservation`.
- Expect error containing `"client not found"`.

---

### Stage 2 — Status Transitions (Cancel & Complete)

**File**: add cases to `setup_test.go` inside `RunServicePureTests`  
**Goal**: Validate the remaining FSM transitions through the service layer, including event emission.

#### UC-05: `ChangeReservationStatus_Cancel_FromPending`
- Create a reservation → `StatusPending`.
- `ChangeReservationStatus(EventCancel)`.
- Assert status is `StatusCancelled`.
- Assert `EventReservationCancelled` was published.

#### UC-06: `ChangeReservationStatus_Complete_FromConfirmed`
- Create a reservation → Confirm → `StatusConfirmed`.
- `ChangeReservationStatus(EventComplete)`.
- Assert status is `StatusCompleted`.
- Assert `EventReservationCompleted` was published.

---

### Stage 3 — Rescheduling Flow

**File**: add case to `setup_test.go` inside `RunServicePureTests`  
**Goal**: Validate the atomic reschedule operation (original → `RESCHEDULED`, new → `PENDING`).

#### UC-07: `CreateReservation_Reschedule`
1. Create original reservation at slot A → `original.ID`, `original.Status = PENDING`.
2. Call `CreateReservation` with `RescheduledFromID: original.ID` at slot B.
3. Assert `newReservation.Status == StatusPending`.
4. Assert `original.Status == StatusRescheduled`.
5. Assert both `EventReservationCreated` and `EventReservationRescheduled` were published.

---

### Stage 4 — Availability with Exceptions

**File**: add cases to `setup_test.go` inside `RunServicePureTests`  

#### UC-08: `ListAvailability_HolidayException`
- Insert `WorkCalendarException{ExceptionType: "HOLIDAY"}`.
- Assert `len(slots) == 0`.

#### UC-09: `ListAvailability_BlockedException`
- Insert `WorkCalendarException{ExceptionType: "BLOCKED"}`.
- Assert no slot overlaps the blocked window.

#### UC-10: `ListAvailability_SpecialHoursException`
- Insert `WorkCalendarException{ExceptionType: "SPECIAL_HOURS"}`.
- Assert slots only within the special window.

---

### Stage 5 — Availability with Break Times

**File**: add case to `setup_test.go` inside `RunServicePureTests`  

#### UC-11: `ListAvailability_BreakTime`
- Setup calendar with `BreakStart`/`BreakFinish`.
- Assert no slot overlaps the break interval.

---

### Stage 6 — Cross-Tenant Isolation

**File**: add cases to `setup_test.go`

#### UC-13: `GetReservation_CrossTenantIsolation`
- Create reservation in Tenant **T1**.
- Call `GetReservation` using Tenant **T2**.
- Assert `ErrNotFound`.

#### UC-14: `ChangeReservationStatus_CrossTenantIsolation`
- Create reservation in Tenant **T1**.
- Call `ChangeReservationStatus` using Tenant **T2**.
- Assert `ErrNotFound`.

---

### Stage 7 — Advanced Availability Logic

**File**: add cases to `setup_test.go`

#### UC-15: `ListAvailability_NoCalendarConfig`
- Call `ListAvailability` for staff with no config.
- Assert `ErrCalendarConfigNotFound`.

#### UC-16: `ListAvailability_InactiveCalendarConfig`
- Insert `WorkCalendarConfig` with `IsActive: false`.
- Call `ListAvailability`.
- Assert `len(slots) == 0`.

#### UC-17: `CreateReservation_SlotOutsideWorkHours`
- Call `CreateReservation` for a slot at midnight (outside 09-17).
- Assert `ErrSlotTaken`.

---

### Stage 8 — Enhanced Transition Logic

**File**: add cases to `setup_test.go`

#### UC-12: `UpsertWeeklyCalendar_CalendarConfigNotFound`
- Call `UpsertWeeklyCalendar` for staff with no config.
- Assert `ErrCalendarConfigNotFound`.

#### UC-18: `ChangeReservationStatus_ConfirmWithPaymentID`
- Call `EventConfirm` with `PaymentID: "pay_123"`.
- Assert `PaymentID` is saved.

---

### Stage 9 — Delegated Methods & Boundary Cases

**File**: add cases to `setup_test.go`

#### UC-19: `ListReservationsByStaff_ViaService`
- Verify delegation and staff filtering.

#### UC-20: `ExpirePendingReservations_NothingToExpire`
- Call with timestamp before all reservations.
- Assert 0 expired.

---

## Verification Steps

```bash
gotest
```

## Linked Documents

- [README.md](../README.md)
- [FSM Diagram](diagrams/fsm.md)
- [Test Coverage Mindmap](diagrams/service_tests.md)
