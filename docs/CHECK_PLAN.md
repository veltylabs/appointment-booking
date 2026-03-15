# Migrate to tinywasm/orm v2 API (fmt.Field)

> **Note**: Previous test coverage plan preserved in [PLAN_TESTS_BACKUP.md](PLAN_TESTS_BACKUP.md).

## Context

The ORM code generator (`ormc`) now produces `Schema() []fmt.Field` (from `tinywasm/fmt`) with individual bool constraint fields instead of the old `[]orm.Field` with bitmask constraints. The `Values()` method is removed; consumers use `fmt.ReadValues(schema, ptrs)` instead.

### Key API Changes

| Old (current) | New (target) |
|---|---|
| `[]orm.Field{Name, Type: orm.TypeText, Constraints: orm.ConstraintPK}` | `[]fmt.Field{Name, Type: fmt.FieldText, PK: true}` |
| `orm.TypeText`, `orm.TypeInt64`, `orm.TypeBool` | `fmt.FieldText`, `fmt.FieldInt`, `fmt.FieldBool` |
| Bitmask constraints (`orm.ConstraintPK \| orm.ConstraintNotNull`) | Bool fields (`PK: true, NotNull: true`) |
| `m.Values() []any` | `fmt.ReadValues(m.Schema(), m.Pointers())` |
| `var Reservation_ = struct{...}` | Consistent `_` suffix (verify all models) |

### Models in scope

- `EmployeeServiceConfig`
- `WorkCalendarConfig`
- `WorkCalendarWeekly`
- `WorkCalendarException`
- `Reservation`

### Target fmt.Field Struct (`tinywasm/fmt`)

```go
type Field struct {
    Name    string
    Type    FieldType // FieldText, FieldInt, FieldFloat, FieldBool, FieldBlob, FieldStruct
    PK      bool
    Unique  bool
    NotNull bool
    AutoInc bool
    Input   string
    JSON    string
}
```

### Generated Code per Struct (`ormc`)

- `TableName() string`, `FormName() string`
- `Schema() []fmt.Field`, `Pointers() []any`
- `T_` metadata struct with typed column constants
- `ReadOneT(qb *orm.QB, model *T)`, `ReadAllT(qb *orm.QB)`

---

## Stage 1 — Regenerate ORM Code

**File**: `model_orm.go` (auto-generated)

1. Update `ormc`: `go install github.com/tinywasm/orm/cmd/ormc@latest`
2. Run `ormc` from project root
3. Verify all 5 models generated with `fmt.Field`, bool constraints, `_` suffix meta structs
4. Verify `Values()` is no longer generated

---

## Stage 2 — Update Handwritten Code

**Files**: `repository.go`, `service.go`

1. Replace any `*Meta.` references with `*_.` if meta struct naming changed
2. Search for `.Values()` calls → replace with `fmt.ReadValues(m.Schema(), m.Pointers())`
3. **Fix anti-pattern** in `repository.go`: replace `err.Error() == "sql: no rows in result set"` with `errors.Is(err, orm.ErrNotFound)`
4. Add `"github.com/tinywasm/fmt"` import where needed

> **Note**: `db.Query()`, `.Where()`, `.Eq()`, `.Gte()`, `.Lte()`, `orm.Eq()`, `db.Tx()`, `ReadAll*`, `ReadOne*` — all unchanged.

---

## Stage 3 — Update Tests

**Files**: `tests/setup_test.go`

1. If tests construct `orm.Field` literals, update to `fmt.Field`
2. Run tests to verify no regressions

---

## Stage 4 — Update go.mod

1. Run `go mod tidy`

---

## Verification

```bash
gotest
```

## Linked Documents

- [ARCHITECTURE.md](ARCHITECTURE.md)
- [SKILL.md](SKILL.md)
- [PLAN_TESTS_BACKUP.md](PLAN_TESTS_BACKUP.md) — Previous test coverage plan
