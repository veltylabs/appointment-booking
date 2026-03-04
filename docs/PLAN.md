# PLAN.md — appointment-booking

Master orchestrator for implementation. **Execute stages strictly in order — do NOT start a stage until all previous stages pass `gotest`.**

---

## ⚠️ Current State (as of restart)

The repository contains only the stub `appointment-booking.go` file. **No stages have been implemented yet.**

Start from **Stage 1**. Before beginning each stage, verify the previous stage compiles and `gotest` passes.

## Related documents
- [SKILL — LLM-friendly summary](SKILL.md)
- [Architecture](ARCHITECTURE.md)
- [Database Diagram](diagrams/database.md)
- [FSM Diagram](diagrams/fsm.md)
- [Sequence Diagrams](diagrams/sequence.md)
- [Reference UI Layout](reference-layout.jpg)

---

## Development Rules

- **Standard Library only** — no external assertion libs in tests (`testing`, `reflect` only). Use `tinywasm/fmt`, `tinywasm/json`, and `tinywasm/time` instead of standard packages for WASM compatibility.
- **Dependency Injection** — `SchedulingService` receives external interfaces (`StaffReader`, `CatalogReader`, `DirectoryReader`). No direct calls to other modules.
- **No Global State** — all external interactions via injected interfaces.
- **Max 500 lines per file** — split by domain if exceeded.
- **gotest** for running tests. Install first if in isolated environment:
  ```bash
  go install github.com/tinywasm/devflow/cmd/gotest@latest
  ```
- **WASM/Stlib Dual Testing Pattern** — use `//go:build wasm` and `//go:build !wasm` separating test files, sharing a common runner.
- **Diagram-Driven Testing (DDT)** — logic flows defined in `docs/diagrams/*.md` MUST have corresponding Integration Tests.
- **Documentation First** — update docs before writing code.

---

## Architecture decisions

| Decision | Resolution |
|---|---|
| Scope | Model + ORM + Service layer |
| Status management | FSM in code — no `reservation_status` table. Tracked via `diagrams/fsm.md` |
| Payment coupling | Soft reference (`payment_id`); payment module calls `ChangeStatus()` |
| Slot granularity | Derived from `employee_service_config.duration_min` and `buffer_min` |
| Timezone | Single source of truth in `workcalendar_config.timezone` (IANA); weekly/exception times are local integers; `reservation_time` is UTC unix. Human readable dates mapped to `LocalStringDate` |
| Audit fields | UnixID encodes UTC creation time — no separate `created_at` needed. Prices stored as snapshots |
| Soft references | `client_id`, `staff_id`, `service_id`, `creator_user_id`, `payment_id` — no physical FK to other modules |
| Rescheduling | Transactional: creates new reservation + marks original as RESCHEDULED (not CANCELLED) inside a DB Transaction |

---

## Diagrams

- [x] `docs/diagrams/fsm.md` — FSM state machine (created, ready for DDT coverage)
- [x] `docs/diagrams/sequence.md` — Sequence diagrams: ListAvailability, CreateReservation, ChangeReservationStatus, ExpirePendingReservations

## Stages

> Execute strictly in order. Mark a stage `[x]` only after `gotest` passes for that stage.

- [ ] [Stage 1 — Models + FSM](PLAN_STAGE_1_MODELS.md)
- [ ] [Stage 2 — ORM + Migrations](PLAN_STAGE_2_ORM.md)
- [ ] [Stage 3 — Service Layer](PLAN_STAGE_3_SERVICE.md)
- [ ] [Stage 4 — MCP Integration](PLAN_STAGE_4_MCP.md)
