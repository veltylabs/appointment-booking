# Sequence Diagrams — appointment-booking

---

## 1. ListAvailability

Shows how the service assembles free time slots from calendar rules, exceptions, and existing reservations.

```mermaid
sequenceDiagram
    participant MCP as MCP Handler
    participant SVC as SchedulingService
    participant CAL as CalendarStore
    participant RES as ReservationStore
    participant CFG as ConfigStore

    MCP->>SVC: ListAvailability(tenantID, staffID, configID, from, to)

    SVC->>CAL: GetCalendarConfig(tenantID, staffID)
    CAL-->>SVC: WorkCalendarConfig{Timezone}

    SVC->>CAL: ListWeeklyCalendar(tenantID, staffID)
    CAL-->>SVC: []WorkCalendarWeekly

    SVC->>CAL: ListExceptions(tenantID, staffID, from, to)
    CAL-->>SVC: []WorkCalendarException

    SVC->>RES: ListReservationsByStaff(tenantID, staffID, from, to)
    note right of RES: filters out CANCELLED,<br/>RESCHEDULED, EXPIRED
    RES-->>SVC: []Reservation

    SVC->>CFG: GetEmployeeServiceConfig(tenantID, configID)
    CFG-->>SVC: EmployeeServiceConfig{DurationMin, BufferMin}

    loop For each day in [from, to]
        SVC->>SVC: find weekly rule for day-of-week
        SVC->>SVC: LocalIntToUnixUTC(day, WorkStart/Finish/Break, Timezone)
        SVC->>SVC: apply exceptions (HOLIDAY > SPECIAL_HOURS > BLOCKED)
        SVC->>SVC: generate slots (duration_min, skip break, enforce buffer)
        SVC->>SVC: remove slots overlapping existing reservations
    end

    SVC-->>MCP: []TimeSlot{StartUTC, EndUTC}
```

---

## 2. CreateReservation (with optional reschedule)

Shows cross-module validation, availability check, and the atomic DB transaction.
The reschedule path marks the original reservation as `RESCHEDULED` (not `CANCELLED`).

```mermaid
sequenceDiagram
    participant MCP as MCP Handler
    participant SVC as SchedulingService
    participant CFG as ConfigStore
    participant DIR as DirectoryReader
    participant STF as StaffReader
    participant CAT as CatalogReader
    participant RES as ReservationStore
    participant FSM as FSM
    participant PUB as EventPublisher

    MCP->>SVC: CreateReservation(cmd)

    SVC->>CFG: GetEmployeeServiceConfig(tenantID, configID)
    CFG-->>SVC: config (StaffID, ServiceID, DurationMin, BufferMin, Price)

    SVC->>DIR: ClientExists(tenantID, clientID)
    DIR-->>SVC: true / ErrInvalidClient

    SVC->>STF: StaffExists(tenantID, config.StaffID)
    STF-->>SVC: true / ErrInvalidStaff

    SVC->>CAT: ServiceExists(tenantID, config.ServiceID)
    CAT-->>SVC: true / ErrInvalidService

    note over SVC: internal ListAvailability call<br/>(see diagram 1)
    SVC->>SVC: verify SlotStartUTC is a free slot
    SVC-->>MCP: ErrSlotTaken (if not free)

    note over SVC,RES: BEGIN DB TRANSACTION

    opt RescheduledFromID is set
        SVC->>RES: GetReservation(tenantID, rescheduledFromID)
        RES-->>SVC: originalReservation{Status, Revision}
        SVC->>FSM: Transition(originalStatus, RESCHEDULE)
        FSM-->>SVC: RESCHEDULED
        SVC->>RES: UpdateReservationStatus(id, RESCHEDULED, revision)
        RES-->>SVC: ok / ErrConflict
    end

    SVC->>SVC: build Reservation{Status:PENDING, snapshots, LocalStringDate/Time}
    SVC->>RES: InsertReservation(newReservation)
    note right of RES: unique constraint on<br/>(staff_id, date, time)<br/>prevents double-booking
    RES-->>SVC: ok / ErrSlotTaken (concurrent race)

    note over SVC,RES: COMMIT TRANSACTION

    SVC->>PUB: Publish(EventReservationCreated, newReservation)
    opt RescheduledFromID was set
        SVC->>PUB: Publish(EventReservationRescheduled, originalReservation)
    end
    note right of PUB: fire-and-forget<br/>errors logged, never fail op

    SVC-->>MCP: Reservation
```

---

## 3. ChangeReservationStatus

Shows FSM enforcement and optimistic concurrency. MCP is the only external entry point.
On `ErrConflict` the MCP handler reloads the reservation and retries with the updated revision.

> **Note on reschedule:** `CreateReservation` does NOT route through this method internally.
> Within the DB transaction it calls `FSM.Transition()` + `RES.UpdateReservationStatus()` directly
> to remain within the same transaction context (see diagram 2).

```mermaid
sequenceDiagram
    participant MCP as MCP Handler
    participant SVC as SchedulingService
    participant RES as ReservationStore
    participant FSM as FSM
    participant PUB as EventPublisher

    MCP->>SVC: ChangeReservationStatus(cmd{ID, Event, ActorID, Revision})

    SVC->>RES: GetReservation(tenantID, id)
    RES-->>SVC: Reservation{Status, Revision}

    SVC->>FSM: Transition(current.Status, cmd.Event)
    FSM-->>SVC: newStatus / ErrInvalidTransition

    opt Event == CONFIRM and PaymentID != ""
        SVC->>SVC: set reservation.PaymentID
    end

    SVC->>RES: UpdateReservationStatus(id, newStatus, actorID, now, cmd.Revision)
    note right of RES: WHERE revision = cmd.Revision<br/>0 rows affected → ErrConflict

    alt ok
        RES-->>SVC: ok
        SVC->>PUB: Publish(domainEvent, updatedReservation)
        note right of PUB: event mapped from cmd.Event<br/>fire-and-forget
        SVC-->>MCP: nil
    else ErrConflict (concurrent update)
        RES-->>SVC: ErrConflict
        SVC-->>MCP: ErrConflict
        note over MCP: reload reservation<br/>and retry with new revision
    end
```

---

## 4. ExpirePendingReservations

Triggered exclusively by an external scheduler via the `expire_pending_reservations` MCP tool.

```mermaid
sequenceDiagram
    participant SCHED as External Scheduler
    participant MCP as MCP Handler
    participant SVC as SchedulingService
    participant RES as ReservationStore
    participant FSM as FSM

    SCHED->>MCP: expire_pending_reservations{tenantID, before}
    MCP->>SVC: ExpirePendingReservations(tenantID, before)

    SVC->>RES: ListReservationsByStatus(tenantID, PENDING, before)
    note right of RES: ReservationTime < before
    RES-->>SVC: []Reservation

    loop For each pending reservation
        SVC->>FSM: Transition(PENDING, EXPIRE)
        FSM-->>SVC: EXPIRED
        SVC->>RES: UpdateReservationStatus(id, EXPIRED, "scheduler", now, revision)
        RES-->>SVC: ok / ErrConflict (skipped, log and continue)
    end

    SVC-->>MCP: count of expired reservations
    MCP-->>SCHED: {expired: N}
```
