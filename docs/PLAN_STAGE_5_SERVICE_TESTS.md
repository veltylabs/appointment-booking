# Stage 1 — Service Layer Tests

← Master: [PLAN.md](PLAN.md)

## Goal

Create the missing test coverage for `service.go` using the Dual Testing Pattern.

---

## Files to create

| File | Purpose |
|---|---|
| `service_front_test.go` | WASM tests — pure FSM/algorithm logic only (`//go:build wasm`) |
| `service_back_test.go` | Backend tests — real `tinywasm/sqlite` in-memory DB (`//go:build !wasm`) |
| `setup_test.go` | Shared test runner and setup. Define mocks for external dependencies: `StaffReader`, `CatalogReader`, `DirectoryReader`, `EventPublisher` |

---

## Testing Plan

1. Follow the **WASM/Stlib Dual Testing Pattern**.
2. Create mocks in `setup_test.go`:
   - `MockStaffReader`
   - `MockCatalogReader`
   - `MockDirectoryReader`
   - `MockEventPublisher`
3. Define the shared unit tests in `RunServicePureTests(t)` inside `setup_test.go`, and call it from `service_front_test.go` and `service_back_test.go`.
4. In `service_back_test.go`, define and use `RunServiceIntegrationTests(t)` using an in-memory `tinywasm/sqlite` DB with auto-migration. Do not mock the database operations; use the real ORM mappings.

---

## Acceptance Criteria

- [ ] Ensure all FSM transitions are thoroughly tested.
- [ ] Cover the `ListAvailability` algorithm with unit tests.
- [ ] Ensure successful concurrency handling and transaction handling for `ChangeReservationStatus`.
- [ ] Achieve a high test coverage (`gotest`).
- [ ] Complete stage when tests seamlessly run on both standard Go (`//go:build !wasm`) and TinyGo/WASM (`//go:build wasm`).
