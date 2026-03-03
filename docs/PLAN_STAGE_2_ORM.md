# Stage 2 — ORM + Migrations

← [Stage 1 — Models](PLAN_STAGE_1_MODELS.md) | Next → [Stage 3 — Service](PLAN_STAGE_3_SERVICE.md)

## Goal

Map structs to DB tables using the **`github.com/tinywasm/orm`** library. Provide CRUD operations for each entity (e.g., using the `ormc` code generator if applicable, or the corresponding methods).

---

## Reference

- **ORM Usage Guide:** [tinywasm/orm SKILL.md](/home/cesar/Dev/Project/tinywasm/orm/docs/SKILL.md) — Follow this document strictly to understand the `ormc` code generator, how to define tags like `db:"pk"` or `db:"-"`, and how to query the DB using the fluent API (`Where().Eq()`) and the generated type-safe functions (e.g., `ReadAllReservation(qb)`).

---

## Files to create

| File | Purpose |
|---|---|
| `model_orm.go` | ORM table definitions + CRUD queries |
| `model_orm_test.go` | Unit tests with in-memory SQLite (Requires `//go:build !wasm` pattern for backend tests) |

---

## 1. model_orm.go

Register each struct with the ORM. Follow the pattern from `business-hours/model_orm.go`.

Tables:
- `employee_service_config`
- `reservation`
- `workcalendar_config`
- `workcalendar_weekly`
- `workcalendar_exception`

Required operations per table:

### employee_service_config
- `InsertEmployeeServiceConfig(cfg EmployeeServiceConfig) error`
- `GetEmployeeServiceConfig(id string) (EmployeeServiceConfig, error)`
- `ListEmployeeServiceConfigByStaff(tenantID, staffID string) ([]EmployeeServiceConfig, error)`
- `UpdateEmployeeServiceConfig(cfg EmployeeServiceConfig) error`

### reservation
- `InsertReservation(r Reservation) error`
- `GetReservation(id string) (Reservation, error)`
- `ListReservationsByStaff(tenantID, staffID string, from, to int64) ([]Reservation, error)`
- `ListReservationsByClient(tenantID, clientID string) ([]Reservation, error)`
- `UpdateReservationStatus(id, status, updatedBy string, updatedAt int64, revision int) error`

*Note: The reservation mapping should ideally include a Unique Constraint on `(staff_id, reservation_date, reservation_time)` taking into account status, but since status is mutable, we will manage atomic locks or DB Transactions via the service during creation.*

**`revision` field lifecycle (optimistic concurrency):**
- Initial value: `0` — set by `InsertReservation`.
- Incremented by 1 on every `UpdateReservationStatus` call.
- `UpdateReservationStatus` MUST include `WHERE revision = cmd.Revision` in its SQL predicate. If 0 rows are affected, return `ErrConflict` (sentinel error defined in this package).
- The caller (service layer) must reload the reservation and retry with the new revision if `ErrConflict` is returned.

**Transactions:**
- Both ORM connections must support Transactions via the driver (e.g., `BeginTx`) for complex FSM+Reschedule events.

### workcalendar_config
Single source of truth for timezone per staff. Must be upserted before inserting weekly rows.
- `UpsertCalendarConfig(cfg WorkCalendarConfig) error`
- `GetCalendarConfig(tenantID, staffID string) (WorkCalendarConfig, error)`

### workcalendar_weekly
- `UpsertWeeklyCalendar(cal WorkCalendarWeekly) error`
- `ListWeeklyCalendar(tenantID, staffID string) ([]WorkCalendarWeekly, error)`

### workcalendar_exception
- `InsertException(exc WorkCalendarException) error`
- `ListExceptions(tenantID, staffID string, from, to int64) ([]WorkCalendarException, error)`
- `DeleteException(tenantID, id string) error`

---

## 2. model_orm_test.go

- Apply WASM/Stlib dual testing pattern in case of isomorphic models, though SQLite testing is typically backend-only (`//go:build !wasm`).
- Declare explicitly as an "Integration test using in-memory SQLite" (satisfies rule Mocking: No I/O). Setup using the **`github.com/tinywasm/sqlite`** library.
- Run auto-migration before each test.
- Test each CRUD operation.
- Test `UpdateReservationStatus` with optimistic concurrency (`revision` mismatch must error).
- Test `GetCalendarConfig` returns `ErrNotFound` when config does not exist.
- Ensure transactions work seamlessly.

---

## Acceptance criteria

- [ ] `gotest` passes
- [ ] Optimistic concurrency on `UpdateReservationStatus` works (wrong revision → error)
- [ ] No raw SQL strings — use ORM API only
