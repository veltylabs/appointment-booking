---
PLAN: "refactor!: migrate github.com/tinywasm -> webtyp.com + view.New(Lister) with a caller-scoped Lister"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 10717474644995475446
PR: https://github.com/veltylabs/appointment_booking/pull/10
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `appointment_booking`: WebTyp rename + new `view.New` API

The framework moved `github.com/tinywasm/*` → `webtyp.com/*` (vanity path, every
module published). `origin/main` of this repo is still entirely on
`github.com/tinywasm/*`. Three jobs:

- **A.** The mechanical rename `github.com/tinywasm/*` → `webtyp.com/*`.
- **B.** `github.com/tinywasm/form/input` → **`webtyp.com/input`** (package split;
  same API).
- **C.** New `webtyp.com/view` `view.New` — `view.WithArgs` was **removed**, and
  this view lists with a `(tenantId, staffId)` scope, so it needs a small
  **caller-scoped `view.Lister`** written in this module.

Module import path **stays** `github.com/veltylabs/appointment_booking`.
Working references from siblings that finished the same migration on `main`:
`github.com/veltylabs/business_hours` (custom `lister.go`),
`github.com/veltylabs/work_schedule` (the `form/input` move).

---

## A. Rename `github.com/tinywasm` → `webtyp.com`

Replace import-path prefix **`github.com/tinywasm/`** → **`webtyp.com/`** in
every `.go` file. Files on `origin/main` that reference it: `fsm.go`,
`model.go`, `model_orm.go`, `ops.go`, `repository.go`, `service.go`, `svg.go`,
`view.go`, and under `tests/`: `availability_runner_test.go`,
`conformance_test.go`, `mocks_test.go`, `ops_test.go`, `repository_test.go`,
`service_back_test.go`, `service_runner_test.go`, `setup_test.go`,
`tenant_isolation_test.go`. Grep to be sure:
`grep -rln 'github.com/tinywasm' --include='*.go' .`. Package selectors do not
change — only the path in the `import` block. `svg.go` uses `webtyp.com/svg` —
straight rename.

### `go.mod`

`origin/main` require block:

```
github.com/tinywasm/ddl v0.0.7
github.com/tinywasm/events v0.0.2
github.com/tinywasm/fmt v0.25.5
github.com/tinywasm/form v0.3.13
github.com/tinywasm/model v0.1.2
github.com/tinywasm/orm v0.11.4
github.com/tinywasm/router v0.1.19
github.com/tinywasm/storage v0.0.2-0.20260717121821-7e528006807f
github.com/tinywasm/svg v0.1.14
github.com/tinywasm/time v0.5.2
github.com/tinywasm/view v0.1.12
github.com/tinywasm/dom v0.13.1 // indirect
github.com/tinywasm/json v0.5.17 // indirect
```

For each still-used `github.com/tinywasm/<X>`:
`go mod edit -droprequire=github.com/tinywasm/<X>` then
`go get webtyp.com/<X>@latest`. Add `go get webtyp.com/input@latest`. Drop
`webtyp.com/form` if nothing imports `"webtyp.com/form"` after **B**
(`grep -rn '"webtyp.com/form"' --include='*.go' .`). Then `go mod tidy`.
`@latest` is authoritative; current tags for reference: `ddl v0.0.15`,
`events v0.0.3`, `fmt v1.0.0`, `input v0.0.6`, `model v0.1.8`, `orm v0.12.1`,
`router v0.1.31`, `storage v0.0.7`, `svg v0.1.x` (use `@latest`), `time v0.5.5`,
`view v0.5.2`, `dom v0.13.10`, `json v0.5.25`. No `github.com/tinywasm/*` in
`go.mod`; no `replace … => ../…` outside this module.

### Docs / config text

`*.md` / `*.yml` / `*.yaml`: `github.com/tinywasm/` → `github.com/webtyp/`.
Prose `TinyWasm`/`TinyWASM` → `WebTyp`. Leave `LICENSE` and upstream "TinyGo".

---

## B. `form/input` → `webtyp.com/input`

`model.go:4` and any other file import `"github.com/tinywasm/form/input"`. That
package no longer exists — it was split into its own module. Change the import
to **`"webtyp.com/input"`**. The selector stays `input.` and every
`input.Text()` / `input.Number()` / `input.Bool()` / … call is byte-for-byte
identical. `model_orm.go` also uses `input.*` in its transport `model.Definition`
blocks — same change. Grep:

```
grep -rn 'form/input\|tinywasm/form/input' --include='*.go' .
```

---

## C. `view.go` — `view.WithArgs` is gone; write a caller-scoped Lister

### The change

`webtyp.com/view`'s `view.New` is now:

```go
func New(l Lister, record model.Model, opts ...Option) Presenter
type Lister interface{ List() ([]model.Model, error) }
```

`view.WithArgs`, `view.WithSaveOp`, `view.WithDeleteOp`, the op-name arg and the
slice-factory arg are all **removed**. `view.WithTitle` /
`view.WithSearchPlaceholder` remain.

`webtyp.com/view` also ships `view.NewCallerLister(caller, view.Ops{…},
newList)` — **but it always calls the list op with `nil` args**, so it CANNOT
carry this view's `(tenantId, staffId)` scope. This module writes its own
`view.Lister` instead.

`origin/main` `view.go` `NewView`:

```go
func NewView(caller router.Caller, tenantId, staffId string) view.Presenter {
	record := &Reservation{}
	return view.New(
		caller, record, OpListReservationsByStaff,
		func() model.ModelSlice { return &ReservationList{} },
		view.WithTitle("Reservas"),
		view.WithArgs(func() model.Encodable {
			return &ListReservationsByStaffArgs{TenantId: tenantId, StaffId: staffId}
		}),
	)
}
```

### Do

1. **New file `lister.go`** — a caller-scoped `view.Lister`. `router.Caller` is
   `Call(op string, args model.Encodable, into model.Decodable, done func(err error))`
   (async; the callback fires with the outcome). `ListReservationsByStaffArgs`
   (`model_orm.go`) is a `model.Encodable`; `*ReservationList` (`model_orm.go`)
   is the `model.Decodable` slice container the transport fills via `Append()`.

   ```go
   package appointmentbooking

   import (
   	"webtyp.com/model"
   	"webtyp.com/router"
   	"webtyp.com/view"
   )

   // reservationLister adapts router.Caller + the staff-scoped list op to
   // view.Lister. view.NewCallerLister is unusable here — it sends nil args,
   // and this list is scoped to (tenantId, staffId).
   type reservationLister struct {
   	caller   router.Caller
   	tenantId string
   	staffId  string
   }

   func (l reservationLister) List() ([]model.Model, error) {
   	out := &ReservationList{}
   	ch := make(chan error, 1)
   	l.caller.Call(
   		OpListReservationsByStaff,
   		&ListReservationsByStaffArgs{TenantId: l.tenantId, StaffId: l.staffId},
   		out,
   		func(err error) { ch <- err },
   	)
   	if err := <-ch; err != nil {
   		return nil, err
   	}
   	rows := make([]model.Model, 0, out.Len())
   	for i := 0; i < out.Len(); i++ {
   		rows = append(rows, out.At(i).(*Reservation))
   	}
   	return rows, nil
   }

   var _ view.Lister = reservationLister{}
   ```

   Confirm `*Reservation` satisfies `model.Model` (it carries `Item()` +
   the orm methods — it does). If `out.At(i)` returns `model.Fielder`, the
   `.(*Reservation)` assertion is correct.

2. **`view.go`** — keep `NewView`'s **exact signature**
   (`func(caller router.Caller, tenantId, staffId string) view.Presenter`).
   Imports `webtyp.com/router` + `webtyp.com/view` stay; drop `webtyp.com/model`
   from `view.go` if nothing there still uses it after the rewrite.

   ```go
   // NewView builds the Reservation Presenter — list-only, scoped to one
   // staff member's schedule (docs/ARCHITECTURE.md §7). Reservations mutate
   // only through the FSM-guarded ChangeReservationStatus op and are never
   // physically deleted, so no Saver/Deleter capability.
   func NewView(caller router.Caller, tenantId, staffId string) view.Presenter {
   	return view.New(
   		reservationLister{caller: caller, tenantId: tenantId, staffId: staffId},
   		&Reservation{},
   		view.WithTitle(titleReservations),
   	)
   }
   ```

   `titleReservations` is a new unexported constant
   (`const titleReservations = "Reservas"`) — no inline literal.
   `(*Reservation).Item()` stays exactly as it is.

3. **`tests/conformance_test.go`** — `view/conformance.FakeCaller` **no longer
   exists** (the conformance package is now `view.Lister`-based:
   `conformance.FakeLister`). Rewrite the test's caller double to
   **`webtyp.com/router/mock`**'s `mock.Caller`, which decodes a canned wire
   response into the `into` target with the real `webtyp/json` codec:

   ```go
   import (
   	"testing"
   	"webtyp.com/json"
   	"webtyp.com/router/mock"
   	"webtyp.com/view"
   	ab "github.com/veltylabs/appointment_booking"
   )

   // …build r1, r2, list := ab.ReservationList{r1, r2} as today…

   body, err := json.Encode(&list)
   if err != nil {
   	t.Fatalf("encode canned list: %v", err)
   }
   caller := &mock.Caller{CannedResult: body}

   pres := ab.NewView(caller, "t1", "staff-1")
   ```

   The rest of the assertions (`Title()`, `Reload()`, `Items()`, `Select`,
   `Selected`, and the `_, ok := pres.(view.Saver)` / `view.Deleter` negative
   checks) stay unchanged in intent — adjust only what the new types require.
   If `json.Encode(&list)` round-trips awkwardly (the slice container decodes
   via `Append()`), a minimal local `router.Caller` stub in the test file that
   sets `*into.(*ab.ReservationList) = list` directly is an acceptable
   fallback — keep it in the test package, not the module.

4. Every other `tests/*.go` that constructs the view or a `FakeCaller` (grep
   `conformance\.` and `NewView(` across `tests/`) gets the same treatment.
   `router/mock.Caller` also records `.Calls` (`[]mock.Call{Op, Args}`) if a
   test needs to assert the op name / args that went out.

---

## Verify

```
grep -rn 'github.com/tinywasm' --include='*.go' --include='go.mod' .   # empty
grep -rn 'form/input' --include='*.go' .                               # empty
grep -rn 'view.WithArgs\|view.WithSaveOp\|view.WithDeleteOp' .         # empty
grep -rn 'conformance.FakeCaller' .                                    # empty
grep -rn '=> \.\./' go.mod                                            # empty
```

## Acceptance

- `go build ./...` → clean.
- `gotest ./...` → all green (vet, race, tests). `TestViewConformance` still
  asserts: title `"Reservas"`, `Reload()` then two projected items with the
  exact `ID`/`Label`/`Description` it checks today, `Select("res-2")` returns
  the record, and the presenter is **neither** `view.Saver` **nor**
  `view.Deleter`.
- No `github.com/tinywasm/*` anywhere in `*.go` / `go.mod`.
- `NewView` still has signature
  `func(caller router.Caller, tenantId, staffId string) view.Presenter`.

## Constraints

- **No behaviour change** — the list is still the staff-scoped
  `OpListReservationsByStaff` call carrying `{TenantId, StaffId}`; still
  list-only (no Save/Delete); title still `"Reservas"`.
- **No hardcoded strings** — `titleReservations`, op names (already in `ops.go`),
  any repeated literal are named constants.
- Keep every `//go:build` tag exactly as-is (backend + WASM shared).
- `reservationLister` lives in the module (`lister.go`); the test double lives
  in the `tests` package. Do not put transport/codec imports (`json`, `jsvalue`)
  in `lister.go` or `view.go` — only the test may name a codec.
- Do not add `From`/`To` to the args unless the old code set them (it did not —
  leave them zero).
