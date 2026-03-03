# appointment-booking Architecture

## 1. Domain Scope

The `appointment-booking` module manages the complete lifecycle of a scheduled service appointment. It is responsible for:
- Configuring which services each staff member offers (duration, price, buffer time).
- Defining staff availability via weekly calendars and one-off exceptions (holidays, special hours, blocked intervals).
- Calculating free time slots and creating reservations with atomic conflict prevention.
- Enforcing reservation state transitions via a Finite State Machine (FSM).

## 2. Core Entities

- **`EmployeeServiceConfig`:** Maps a staff member to a service item, defining duration, buffer time, and price override. The source of truth for slot granularity.
- **`Reservation`:** The appointment itself. Stores snapshots of staff, service, price, and currency at creation time for financial auditability — these never change even if the source data is later modified.
- **`WorkCalendarConfig`:** One row per staff member. Single source of truth for the IANA timezone of the staff calendar. Must exist before weekly rows can be inserted.
- **`WorkCalendarWeekly`:** Recurring weekly schedule for a staff member (one row per working day). Defines local working hours and break. Does not carry timezone — inherits it from `WorkCalendarConfig`.
- **`WorkCalendarException`:** One-off overrides for a specific date: `HOLIDAY` (no availability), `SPECIAL_HOURS` (different hours), or `BLOCKED` (interval subtracted from available window).

## 3. Finite State Machine (FSM)

Reservation status transitions are enforced in code — there is no `reservation_status` DB table.

See: [FSM Diagram](diagrams/fsm.md)

Key decisions:
- `RESCHEDULED` is a distinct terminal state (not `CANCELLED`) to preserve audit trail clarity in analytics.
- `EXPIRED` is triggered exclusively by an external scheduler via the `expire_pending_reservations` MCP tool — the module does not run background goroutines.

## 4. Architectural Patterns

1. **Dependency Injection:** `SchedulingService` receives three external readers (`StaffReader`, `CatalogReader`, `DirectoryReader`) and one `EventPublisher` at construction. No global state, no direct imports from other modules.

2. **Direct ORM access (no store interfaces):** The service struct holds `*orm.DB` directly and calls ORM functions from `model_orm.go`. There are no intermediate `ReservationStore`, `CalendarStore`, or `ConfigStore` interfaces. This keeps the internal boundary thin and allows tests to exercise the real `tinywasm/orm` + `tinywasm/sqlite` stack with an in-memory database — catching real constraint and concurrency bugs instead of hiding them behind mocks. Only cross-module interfaces (`StaffReader`, `CatalogReader`, `DirectoryReader`, `EventPublisher`) are mockable.

3. **Soft References (no physical FK):** `client_id`, `staff_id`, `service_id`, `creator_user_id`, and `payment_id` reference entities in other modules by ID only. Cross-module existence is validated at the application layer via injected readers, not via DB constraints.

4. **Snapshotting:** Price, currency, duration, staff ID, and service ID are snapshotted at reservation creation. Downstream changes to catalog or staff data do not alter existing reservations.

5. **Local Integer Time + IANA Timezone (Single Source of Truth):** Working hours in `WorkCalendarWeekly` are stored as local integers (e.g., `900 = 09:00`). The IANA timezone is stored exclusively in `WorkCalendarConfig` (one row per staff) — `WorkCalendarWeekly` and `WorkCalendarException` do not carry timezone fields. This prevents per-row timezone inconsistency by construction. The `ListAvailability` algorithm loads `WorkCalendarConfig` first to obtain the timezone, then converts local boundaries to Unix UTC using `tinywasm/time`. This design ensures recurring schedules remain correct across DST transitions.

6. **Optimistic Concurrency:** `Reservation.revision` is incremented on each status update. `UpdateReservationStatus` enforces `WHERE revision = N` — a mismatch returns `ErrConflict`, preventing silent overwrites.

7. **Atomic Reschedule:** Rescheduling is not a status — it is a transactional operation: create new reservation + mark original as `RESCHEDULED` within a single DB transaction.

## 5. Identity Contract & RBAC

This module does **not** implement authorization or role-based access control. It operates under the following contract:

- `actorID` (passed in `CreateReservationCmd` and `ChangeStatusCmd`) is a plain string — already authenticated and authorized by the caller.
- The MCP gateway or middleware layer is responsible for verifying that the authenticated user has permission to perform the operation **before** invoking the MCP tool.
- This module stores `actorID` as an audit field (`creator_user_id`, `updated_by`) only.
- **RBAC belongs to a separate IAM module.** Changes to roles or permissions require no changes to this module.

## 6. Event Publishing & Inter-Module Communication

This module communicates outbound via an injected `EventPublisher` interface. After each successful state mutation, it publishes a domain event:

| Operation | Event constant |
|---|---|
| `CreateReservation` | `appointment.reservation.created` |
| `ChangeStatus` CONFIRM | `appointment.reservation.confirmed` |
| `ChangeStatus` CANCEL | `appointment.reservation.cancelled` |
| `ChangeStatus` COMPLETE | `appointment.reservation.completed` |
| `ChangeStatus` NO_SHOW | `appointment.reservation.no_show` |
| `ChangeStatus` EXPIRE | `appointment.reservation.expired` |
| Reschedule (original) | `appointment.reservation.rescheduled` |

**Rules:**
- Event publishing is **fire-and-forget** — errors are logged but never fail the operation.
- Passing `nil` as the publisher safely disables events (useful in tests or CLI tools).
- The `EventPublisher` implementation is decided by the monolith's composition root, not by this module.

**Possible implementations** (the module doesn't know or care which):
- In-process Go channel (single-process monolith)
- Synchronous callback function (integration tests)
- Message queue adapter (NATS, Redis Pub/Sub) for distributed deployments

## 7. Composition Root — Coupling to the Monolith

This module is a library. The consuming application wires all dependencies at startup in a single composition root (`main.go`):

```go
// Each module implements the interface required by appointment-booking.
// No import cycles — appointment-booking never imports staff, catalog, or directory modules.

staffSvc     := staffmodule.New(db)      // implements StaffReader
catalogSvc   := catalogmodule.New(db)    // implements CatalogReader
directorySvc := directorymodule.New(db)  // implements DirectoryReader
eventBus     := eventbus.New()           // implements EventPublisher

// db is *orm.DB — shared with the rest of the monolith or module-specific.
scheduling := appointmentbooking.New(db, appointmentbooking.Deps{
    Staff:     staffSvc,
    Catalog:   catalogSvc,
    Directory: directorySvc,
    Publisher: eventBus,  // nil = no events
})

// Register MCP tools
appointmentbooking.Register(mcpServer, scheduling)
```

The Dependency Inversion Principle is enforced for cross-module concerns: `appointment-booking` defines `StaffReader`, `CatalogReader`, `DirectoryReader`, and `EventPublisher` — other modules provide adapter implementations. Internal DB operations go directly through `*orm.DB` (no indirection layer needed). No module imports another directly.

## 8. Availability Calculation

Free slots are derived at query time from the intersection of:
- Weekly calendar rules (recurring working hours)
- One-off exceptions (override or subtract from weekly rules)
- Existing non-terminal reservations (block occupied intervals including buffer time)

Exception priority: `HOLIDAY` > `SPECIAL_HOURS` > `BLOCKED`.

See: [ListAvailability algorithm](PLAN_STAGE_3_SERVICE.md#4-listavailability-algorithm)

Also see: [Composition Root Sequence Diagram](diagrams/sequence.md)

## 9. Related Documents

- [SKILL — LLM-friendly summary](SKILL.md)
- [Database Diagram](diagrams/database.md)
- [FSM Diagram](diagrams/fsm.md)
- [Sequence Diagrams](diagrams/sequence.md) — ListAvailability, CreateReservation, ChangeReservationStatus, ExpirePendingReservations
- [Implementation Plan](PLAN.md)
