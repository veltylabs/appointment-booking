# PLAN.md — appointment-booking tests completion

## Goal

Complete the missing testing files for the `service.go` logic that were skipped in the previous implementation phase. The service layer currently lacks test coverage for `ListAvailability`, FSM transitions, concurrency handling, and basic CRUD operations.

---

## Development Rules

- **gotest** for running tests. Install first if in isolated environment:
  ```bash
  go install github.com/tinywasm/devflow/cmd/gotest@latest
  ```
- **WASM/Stlib Dual Testing Pattern** — use `//go:build wasm` and `//go:build !wasm` separating test files, sharing a common runner.
- **Diagram-Driven Testing (DDT)** — logic flows defined in `docs/diagrams/*.md` MUST have corresponding Integration Tests.
- **Standard Library only** — no external assertion libs in tests (`testing`, `reflect` only).
- **Mocks (No I/O)** — Use Mocks for all external interfaces in the service.

---

## ⚠️ Current State

`service.go` is implemented but the required test files (`service_front_test.go`, `service_back_test.go`, and `setup_test.go`) are missing. Test coverage string currently at ~36%.

---

## Stages

- [ ] [Stage 1 — Service Layer Tests](PLAN_STAGE_5_SERVICE_TESTS.md)
