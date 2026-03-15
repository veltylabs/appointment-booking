# Migrate appointment-booking to tinywasm/orm v2 + MCP API update

## Context

Two breaking changes need to be addressed:

1. **ORM v2**: `orm.Field` → `fmt.Field` with bool constraints, `Values()` removed
2. **MCP API**: `mcp.NewTool` renamed to `mcp.NewProtocolTool`

Dependencies are already at latest versions in go.mod (orm v0.3.2, sqlite v0.1.12).

---

## Stage 1 — Regenerate ORM Code

**File**: `model_orm.go` (auto-generated)

1. Install ormc: `go install github.com/tinywasm/orm/cmd/ormc@latest`
2. Run `ormc` from project root
3. Verify generated file uses `fmt.Field` with bool constraints
4. Verify `Values()` is no longer generated

---

## Stage 2 — Fix mcp.go

**File**: `mcp.go`

### 2.1 — Replace mcp.NewTool → mcp.NewProtocolTool

The function `mcp.NewTool` was renamed to `mcp.NewProtocolTool`. Replace ALL occurrences:

```
mcp.NewTool(  →  mcp.NewProtocolTool(
```

There are ~10 calls. The signature and options (`WithDescription`, `WithString`, `WithNumber`, `WithBoolean`, `Required`, `Description`) are all unchanged — only the function name changed.

### 2.2 — Simplify argument extraction (optional but recommended)

`CallToolRequest` now provides helper methods. You can simplify:

```go
// BEFORE:
args, ok := req.Params.Arguments.(map[string]any)
if !ok {
    return mcp.NewToolResultError("invalid arguments"), nil
}
tenantID := args["tenant_id"].(string)

// AFTER:
tenantID := req.GetString("tenant_id", "")
```

Available helpers: `req.GetString(key, default)`, `req.GetInt(key, default)`, `req.GetBool(key, default)`, `req.GetArguments()`.

This is optional — the existing `req.Params.Arguments.(map[string]any)` pattern still works.

---

## Stage 3 — Fix anti-pattern in repository.go

**File**: `repository.go`

Replace the string comparison error check with proper error matching:

```go
// BEFORE:
err.Error() == "sql: no rows in result set"

// AFTER:
errors.Is(err, orm.ErrNotFound)
```

There are 2 occurrences in `repository.go` (around lines 243 and 246).

Also in `service.go` line ~565:
```go
// BEFORE:
if errors.Is(err, orm.ErrNotFound) || err.Error() == "sql: no rows in result set" {

// AFTER:
if errors.Is(err, orm.ErrNotFound) {
```

---

## Stage 4 — Update go.mod

1. Run `go mod tidy`

---

## Verification

```bash
gotest
```
