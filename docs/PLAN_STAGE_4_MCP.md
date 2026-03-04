# Stage 4 — MCP Integration

← [Stage 3 — Service](PLAN_STAGE_3_SERVICE.md) | Master: [PLAN.md](PLAN.md)

> **Prerequisite:** Stages 1, 2, and 3 must pass `gotest` before starting this stage.

## Goal

Expose `SchedulingService` operations as MCP tools utilizing the **`github.com/tinywasm/mcp`** library. Follow the pattern from `business-hours/mcp.go`.

---

## Files to create

| File | Purpose |
|---|---|
| `mcp.go` | MCP tool registration + handlers |
| `mcp_test.go` | Integration tests (add `//go:build !wasm` since MCP server targets backend) |

---

## 1. MCP tools to expose

| Tool name | Maps to | Notes |
|---|---|---|
| `list_availability` | `SchedulingService.ListAvailability()` | |
| `create_reservation` | `SchedulingService.CreateReservation()` | |
| `get_reservation` | `SchedulingService.GetReservation()` | |
| `list_reservations_by_staff` | `SchedulingService.ListReservationsByStaff()` | |
| `list_reservations_by_client` | `SchedulingService.ListReservationsByClient()` | |
| `change_reservation_status` | `SchedulingService.ChangeReservationStatus()` | |
| `upsert_calendar_config` | `SchedulingService.UpsertCalendarConfig()` | Sets IANA timezone for a staff member — must be called before `upsert_weekly_calendar` |
| `upsert_weekly_calendar` | `SchedulingService.UpsertWeeklyCalendar()` | Must surface `ErrCalendarConfigNotFound` with actionable message |
| `add_calendar_exception` | `SchedulingService.AddException()` | |
| `remove_calendar_exception` | `SchedulingService.RemoveException()` | |
| `expire_pending_reservations` | `SchedulingService.ExpirePendingReservations()` | Called by external scheduler/cron |

### `expire_pending_reservations` — trigger mechanism

This tool is the **only external entry point** that can trigger the `EXPIRE` FSM event. It is designed to be called by an external scheduler (e.g., a cron job, a platform-level task runner, or another MCP-enabled agent) with a Unix UTC timestamp threshold.

Input:
```json
{ "tenant_id": "...", "before": 1712000000 }
```

The handler calls `ExpirePendingReservations(ctx, tenantID, before)` and returns the count of expired reservations. This module does **not** run background goroutines — expiration is always externally triggered.

---

## 2. mcp.go structure

Follow `business-hours/mcp.go` pattern leveraging `github.com/tinywasm/mcp`:
- One `Register(server *mcp.Server)` function that registers all tools
- Each handler: parse input -> call service -> return JSON response
- Errors returned as MCP error responses using the `tinywasm/mcp` error structures (not panics)
- `ErrTimezoneMismatch` and `ErrSlotTaken` must be mapped to descriptive MCP user-facing error messages (not internal codes)

---

## 3. mcp_test.go

- Test each tool handler with valid and invalid inputs
- Use mock `SchedulingService` (interface-based)
- Verify JSON response shape for success and error cases
- Test `expire_pending_reservations` with `before` in the past and future

### ⚠️ Critical: Test input construction pattern

When building `CallToolRequest` in tests, **always** set `Params.Arguments` as a `map[string]any` with `snake_case` keys directly — **never** as a Go struct marshaled through JSON tags. This mirrors exactly what a real MCP/LLM client sends over the wire and is the canonical pattern in the `tinywasm/mcp` ecosystem.

The `BindArguments` flow in `provider.go` does:
```
json.Marshal(Arguments) → json.Unmarshal(→ map[string]any)
```
If you pass a struct, the resulting map keys depend on the struct's JSON tags, which may not match the snake_case keys the service handler expects, causing arguments to silently default to zero values and bypassing intended error branches.

**Correct pattern:**
```go
// UpsertWeeklyCalendar_no_config — missing config must return ErrCalendarConfigNotFound
req.Params.Arguments = map[string]any{
    "tenant_id": "t1",
    "staff_id":  "",  // empty → triggers ErrCalendarConfigNotFound
}

// CreateReservation_slot_taken — slot already booked
req.Params.Arguments = map[string]any{
    "tenant_id":                  "t1",
    "client_id":                  "c1",
    "employee_service_config_id": "esc1",
    "slot_start_utc":             int64(1712000000),
}
```

**Do NOT** change JSON tags on structs to match. **Do NOT** add `json.Marshal` gymnastics in test setup. The fix is always: use `map[string]any` with snake_case keys as test input.

---

## Acceptance criteria

- [ ] `gotest` passes
- [ ] All 11 tools registered and reachable via MCP server
- [ ] Error responses follow MCP error format
- [ ] `ErrCalendarConfigNotFound` and `ErrSlotTaken` produce descriptive user-facing messages
- [ ] `expire_pending_reservations` documented as scheduler-triggered (not self-triggered)
- [ ] `gopush 'implement appointment-booking module'` succeeds
