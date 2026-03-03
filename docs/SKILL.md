# SKILL: appointment-booking module

## Overview

The `appointment-booking` module manages the full lifecycle of scheduled service appointments in a multi-tenant clinic/business context. It owns: staff calendar configuration, availability calculation, and reservation state transitions.

---

## Core Constraints & Rules

- **FSM-only status changes:** `Reservation.Status` MUST only change via `FSM.Transition(current, event)`. Never set status directly. Valid events: `CONFIRM`, `CANCEL`, `COMPLETE`, `NO_SHOW_EVENT`, `EXPIRE`, `RESCHEDULE`.
- **RESCHEDULED ≠ CANCELLED:** When a reservation is replaced by a new one, the original is marked `RESCHEDULED` (not `CANCELLED`) for audit trail integrity. These are different terminal states.
- **Timezone is in WorkCalendarConfig:** `WorkCalendarWeekly` and `WorkCalendarException` have NO timezone field. Always load `WorkCalendarConfig` first to get the IANA timezone. Working hours are local integers (e.g., `900 = 09:00`), converted to UTC at query time via `LocalIntToUnixUTC`.
- **Snapshotting:** At reservation creation, price, currency, duration, staffID, and serviceID are snapshotted. Never mutate snapshot fields after creation.
- **No RBAC here:** This module trusts `actorID` as an already-authorized string. Authorization is enforced by the MCP gateway before reaching the service. This module only stores actorID as an audit field.
- **EventPublisher is fire-and-forget:** After each successful state mutation, publish a domain event via the injected `EventPublisher`. Publish errors are logged and never fail the operation. `nil` publisher is safe.
- **MCP is the only external entry point.** The service is never called directly except by the MCP handler layer.
- **No cross-module imports.** External dependencies are accessed only via injected interfaces: `StaffReader`, `CatalogReader`, `DirectoryReader`.
- **`tinywasm` packages only** for WASM compatibility: use `tinywasm/fmt`, `tinywasm/time`, `tinywasm/json` — never standard library equivalents.

---

## Primary Entities

- **`WorkCalendarConfig`** — One row per staff. Holds IANA timezone. Must exist before `WorkCalendarWeekly` rows can be upserted. Sentinel error: `ErrCalendarConfigNotFound`.
- **`WorkCalendarWeekly`** — Recurring weekly hours (one row per working day per staff). No timezone field.
- **`WorkCalendarException`** — One-off date overrides: `HOLIDAY` (no slots), `SPECIAL_HOURS` (replace hours), `BLOCKED` (subtract interval). Priority: HOLIDAY > SPECIAL_HOURS > BLOCKED.
- **`EmployeeServiceConfig`** — Maps a staff member to a catalog service. Source of `DurationMin`, `BufferMin`, `PriceOverride`. Must be active for availability and reservations.
- **`Reservation`** — The appointment. Holds FSM status, snapshots, and `RescheduledFromID` for lineage tracking.

---

## Injected Interfaces (constructor parameters)

The service holds `*orm.DB` directly — no intermediate store interfaces. Only cross-module dependencies are injected:

```go
type Deps struct {
    Staff     StaffReader     // provided by staff module
    Catalog   CatalogReader   // provided by catalog module
    Directory DirectoryReader // provided by directory module
    Publisher EventPublisher  // nil = events disabled
}

// Constructor: db is *orm.DB passed directly.
func New(db *orm.DB, deps Deps) SchedulingService
```

---

## Domain Events Published

| Event constant | When |
|---|---|
| `appointment.reservation.created` | After `CreateReservation` commits |
| `appointment.reservation.rescheduled` | For the original reservation during reschedule |
| `appointment.reservation.confirmed` | After CONFIRM transition |
| `appointment.reservation.cancelled` | After CANCEL transition |
| `appointment.reservation.completed` | After COMPLETE transition |
| `appointment.reservation.no_show` | After NO_SHOW transition |
| `appointment.reservation.expired` | After EXPIRE transition |

---

## Key Error Sentinels

| Error | When |
|---|---|
| `ErrSlotTaken` | Slot not available or concurrent booking race |
| `ErrConflict` | Optimistic concurrency mismatch on `UpdateReservationStatus` |
| `ErrCalendarConfigNotFound` | `UpsertWeeklyCalendar` called before `UpsertCalendarConfig` |
| `ErrInvalidTransition` | FSM rejects the event for the current status |

---

## Composition Root (how to wire this module)

```go
scheduling := appointmentbooking.New(db, appointmentbooking.Deps{
    Staff:     staffmodule.New(db),     // implements StaffReader
    Catalog:   catalogmodule.New(db),   // implements CatalogReader
    Directory: directorymodule.New(db), // implements DirectoryReader
    Publisher: eventBus,                // nil = events disabled
})
appointmentbooking.Register(mcpServer, scheduling)
```

---

## Available MCP Tools (11 total)

`list_availability`, `create_reservation`, `get_reservation`, `list_reservations_by_staff`, `list_reservations_by_client`, `change_reservation_status`, `upsert_calendar_config`, `upsert_weekly_calendar`, `add_calendar_exception`, `remove_calendar_exception`, `expire_pending_reservations`

> `expire_pending_reservations` is the **only trigger for the EXPIRE FSM event**. It must be called by an external scheduler — the module has no internal background process.

---

## See Also

- [Architecture](ARCHITECTURE.md) — full design rationale
- [FSM Diagram](diagrams/fsm.md) — state machine
- [Sequence Diagrams](diagrams/sequence.md) — system flows
- [Database Diagram](diagrams/database.md)
