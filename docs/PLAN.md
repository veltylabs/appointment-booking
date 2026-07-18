---
PLAN: "feat: appointment_booking joins the reusable-module harness (OpModule, IDGenerator, events.Publisher, ddl, storage/mem tests)"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: **agents-workflow**.

# PLAN — appointment_booking: adopt the reusable-module harness

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`appointment_booking`).
Plan autocontenido: todo contrato, regla y ejemplo relevante está inline. Lee también `AGENTS.md`
(raíz de este repo) antes de tocar nada — declara la whitelist/blacklist de imports completa que este
plan aplica; este documento **no** repite el contrato completo de `model.Kind`, solo lo que cambia.

## 0. Qué está pasando y por qué

Este módulo depende hoy de **implementaciones concretas** en lugar de los puertos del ecosistema
tinywasm: transporte (`tinywasm/mcp`), generador de ID (`tinywasm/unixid`), codec (`tinywasm/json`), y
declara su propio `EventPublisher` (duplicando `events.Publisher`) sobre un tipo no permitido
(`tinywasm/context`). Además tiene un fork local (`internal/tinytime`) de `github.com/tinywasm/time`
con un `replace` en `go.mod` — la forma exacta de anti-patrón que el blacklist de `AGENTS.md` prohíbe.
La implementación de referencia que ya validó el patrón correcto es `github.com/veltylabs/item_catalog`
(mismo directorio padre que este repo) — este plan replica ese patrón aquí.

Este plan **consolida y reemplaza** tres documentos previos de este mismo repo:
`docs/PLAN_MODEL_MIGRATION.md` (contenido reusado verbatim en la Etapa 1, con versiones re-apuntadas),
`docs/PLAN_REMOVE_TINYTIME_SHIM.md` (contenido reusado en la Etapa 0), y el antiguo `docs/PLAN.md`
(que era solo un índice apuntando a los dos anteriores — ese patrón de índice se retira: este
documento es el plan completo, no un índice). `docs/PLAN_TESTS_BACKUP.md` es un backlog de tests
**independiente y de vida más larga** (no una migración de una sola vez) — se mantiene como archivo
separado, referenciado en la §11 de este plan; ver esa sección antes de tocarlo.

## 1. Estado actual verificado — no asumir, esto ES el código real de este repo

- `go.mod` (raíz): `tinywasm/mcp v0.1.1`, `tinywasm/orm v0.7.1`, `tinywasm/sqlite v0.2.0`,
  `tinywasm/unixid v0.2.23`, `tinywasm/time v0.4.0` + `replace github.com/tinywasm/time =>
  ./internal/tinytime`. `internal/tinytime/` existe en disco (confirmado, 11 archivos incluyendo un
  `go.mod` propio) — el shim **no** ha sido eliminado todavía.
- `service.go`: declara `EventPublisher interface { Publish(ctx *tinyctx.Context, event string,
  payload any) error }` (import `tinyctx "github.com/tinywasm/context"`), usa `unixid.NewUnixID()`
  inline en `CreateReservation`, y expone `type SchedulingService interface { ... }` con **todos** sus
  métodos tomando `ctx *tinyctx.Context` como primer parámetro — parámetro que nunca se lee (ni
  `.Value()`, ni `.Method()`, ni nada), solo se reenvía a `s.pub.Publish(ctx, ...)`.
- `repository.go`: `NewRepository` llama `db.CreateTable(t)` (API vieja de `*orm.DB`, ya removida en
  las versiones de `orm` a las que este plan re-apunta — ver §2 y §9) e importa `unixid` directo en
  `InsertReservation`, `InsertException`, `InsertEmployeeServiceConfig`, y en las ramas de creación de
  `UpsertCalendarConfig`/`UpsertWeeklyCalendar`.
- `mcp.go`: `ReservationProvider`/`CalendarProvider` implementan `mcp.ToolProvider` — 6 + 5 = 11
  tools/ops, import directo de `tinywasm/mcp`, `tinywasm/context`, `tinywasm/json`.
- `tests/`: `tests/repository_test.go` y `tests/service_back_test.go` abren
  `sqlite.Open(":memory:")` directo; `tests/mcp_test.go` testea contra `mcp.ToolProvider`;
  `tests/mocks_test.go` declara `MockEventPublisher.Publish(ctx *tinyctx.Context, event string,
  payload any) error`; `tests/service_runner_test.go` importa `strings` (stdlib prohibido, ver
  `AGENTS.md`); todos los archivos en `tests/` que llaman al servicio pasan `ctx :=
  tinyctx.Background()` como primer argumento a cada método.

## 2. Versiones objetivo — re-apuntadas a lo que `item_catalog` usa HOY

`docs/PLAN_MODEL_MIGRATION.md` apuntaba a `model@v0.0.14`/`orm@v0.9.28`/`form@v0.2.15` — esas
versiones están **obsoletas**; verificado directamente contra `go.mod` de
`veltylabs/item_catalog` (implementación de referencia) al momento de escribir este plan:

```
github.com/tinywasm/events v0.0.2
github.com/tinywasm/fmt    v0.25.3
github.com/tinywasm/form   v0.2.16
github.com/tinywasm/model  v0.0.16
github.com/tinywasm/orm    v0.11.1
github.com/tinywasm/router v0.1.15
github.com/tinywasm/time   v0.5.0
github.com/tinywasm/view   v0.1.1
github.com/tinywasm/ddl    v0.0.4   (directa — hoy transitiva en item_catalog; este módulo SÍ migra
                                      schema en New(), así que la declara como dependencia directa)
```

`github.com/tinywasm/storage` llega transitivamente vía `orm` (no la declares directa a menos que
`go mod tidy` lo exija). **`github.com/tinywasm/sqlite`/`tinywasm/mcp`/`tinywasm/json`/
`tinywasm/unixid`/`tinywasm/context` no aparecen en el `go.mod` final** (ver criterio de aceptación,
§10).

---

## Etapa 0 — Eliminar el shim `internal/tinytime`

**Estado: pendiente** (confirmado por `find`/`grep` en §1 — el directorio y el `replace` existen
hoy). Prerequisito ya satisfecho: `Weekday`, `MidnightUTC`, y `LocalMinutesToUnixUTC` existen en el
`github.com/tinywasm/time` real publicado (`v0.5.0`, verificado contra el código fuente de ese
paquete). `service.go` ya usa exactamente esos tres símbolos (líneas 145, 212, 359) — no requiere
cambios de lógica, solo de import/dependencia.

**Pasos:**

1. `rm -rf internal/tinytime` (elimina el directorio completo, incluyendo su `go.mod`/`go.sum`/
   `LICENSE`/`README.md` propios).
2. En `go.mod`, elimina la línea:
   ```
   replace github.com/tinywasm/time => ./internal/tinytime
   ```
3. Actualiza el `require` de `tinywasm/time` a `v0.5.0` (ver §2).
4. `go get github.com/tinywasm/time@v0.5.0 && go mod tidy` (puede diferirse a un solo `go mod tidy`
   final tras todas las etapas — ver §9).

**Criterio:**
```
find . -path ./internal -prune -o -iname "*tinytime*" -print   # sin resultados fuera de comentarios
grep -rn "internal/tinytime" .                                  # vacío
grep -n "replace" go.mod                                        # vacío
```

---

## Etapa 1 — `model.go` → `model.Definition`

Contenido reusado (verificado línea por línea contra el `model_orm.go` actual de este repo) de
`docs/PLAN_MODEL_MIGRATION.md`, con las versiones de §2 en vez de las obsoletas del documento
original. El contrato completo de `model.Kind`/`model.Field`/`model.Definition` está documentado en
`AGENTS.md` (raíz de este repo) — no se repite aquí.

### 1.1 Advertencia de nombres de columna — leer antes de tocar `Reservation`

Las columnas actuales para `StaffIDSnapshot`/`ServiceIDSnapshot` son **`staff_idsnapshot`** /
**`service_idsnapshot`** (sin guión bajo entre "id" y "snapshot" — irregularidad histórica, ya así en
producción). El nombre de columna se escribe explícito en `Field.Name` — preserva exactamente
`"staff_idsnapshot"` / `"service_idsnapshot"`. El identificador Go que `ormc` derive de ese nombre
puede NO ser `StaffIDSnapshot`/`ServiceIDSnapshot` exactos (conversión pura snake→PascalCase de un
token irregular como `idsnapshot`). **Verifica el nombre de campo generado tras correr `ormc`** y
ajusta los usos en `service.go`/`repository.go` al nombre que salga — **nunca renombres la columna**
para forzar que coincida con el nombre Go deseado.

### 1.2 Estado actual (`model.go` completo, a reemplazar íntegro)

```go
package appointmentbooking

// EmployeeServiceConfig maps per-staff-per-service durations and overrides.
type EmployeeServiceConfig struct {
	ID              string `db:"pk"`
	TenantID        string
	StaffID         string
	ServiceID       string
	DurationMin     int64
	BufferMin       int64
	PriceOverride   float64
	PaymentRequired bool
	IsActive        bool
}

// WorkCalendarConfig is the single source of truth for timezone per staff.
type WorkCalendarConfig struct {
	ID       string `db:"pk"`
	TenantID string
	StaffID  string
	Timezone string
	IsActive bool
}

// WorkCalendarWeekly defines recurring weekly hours for a staff member.
type WorkCalendarWeekly struct {
	ID          string `db:"pk"`
	TenantID    string
	StaffID     string
	DayOfWeek   int64
	WorkStart   int64
	WorkFinish  int64
	BreakStart  int64
	BreakFinish int64
	IsActive    bool
}

// WorkCalendarException overrides working hours for a specific date.
type WorkCalendarException struct {
	ID            string `db:"pk"`
	TenantID      string
	StaffID       string
	SpecificDate  int64
	ExceptionType string
	StartTime     int64
	EndTime       int64
	Notes         string
}

// Reservation is the core booking record.
type Reservation struct {
	ID                      string `db:"pk"`
	TenantID                string
	ClientID                string
	CreatorUserID           string
	EmployeeServiceConfigID string
	StaffIDSnapshot         string
	ServiceIDSnapshot       string
	DurationMinSnapshot     int64
	PriceSnapshot           float64
	CurrencySnapshot        string
	ReservationDate         int64
	ReservationTime         int64
	LocalStringDate         string
	LocalStringTime         string
	Status                  string
	RescheduledFromID       string
	PaymentID               string
	Notes                   string
	UpdatedAt               int64
	UpdatedBy               string
	Revision                int64
}

// ormc:formonly
type TimeSlot struct {
	StartUTC int64
	EndUTC   int64
}

// ormc:formonly
type createReservationArgs struct {
	TenantID                string
	ClientID                string
	CreatorUserID           string
	EmployeeServiceConfigID string
	SlotStartUTC            int64
	Notes                   string
	RescheduledFromID       string
}

// ormc:formonly
type getReservationArgs struct {
	TenantID string
	ID       string
}

// ormc:formonly
type listReservationsByStaffArgs struct {
	TenantID string
	StaffID  string
	From     int64
	To       int64
}

// ormc:formonly
type listReservationsByClientArgs struct {
	TenantID string
	ClientID string
}

// ormc:formonly
type changeReservationStatusArgs struct {
	TenantID  string
	ID        string
	Event     string
	ActorID   string
	PaymentID string
	Revision  int64
}

// ormc:formonly
type expirePendingReservationsArgs struct {
	TenantID string
	Before   int64
}

// ormc:formonly
type upsertCalendarConfigArgs struct {
	TenantID string
	StaffID  string
	Timezone string
	IsActive bool
}

// ormc:formonly
type upsertWeeklyCalendarArgs struct {
	TenantID    string
	StaffID     string
	DayOfWeek   int64
	WorkStart   int64
	WorkFinish  int64
	BreakStart  int64
	BreakFinish int64
	IsActive    bool
}

// ormc:formonly
type addCalendarExceptionArgs struct {
	TenantID      string
	StaffID       string
	SpecificDate  int64
	ExceptionType string
	StartTime     int64
	EndTime       int64
	Notes         string
}

// ormc:formonly
type removeCalendarExceptionArgs struct {
	TenantID    string
	ExceptionID string
}

// ormc:formonly
type listAvailabilityArgs struct {
	TenantID string
	StaffID  string
	ConfigID string
	From     int64
	To       int64
}
```

### 1.3 Estado objetivo (`model.go` reescrito completo)

Los 5 structs con rol DB de abajo (`EmployeeServiceConfig` … `Reservation`) **no llevan ningún
widget**: usa Kinds base (`model.X()`). En los 12 transport, la política es por rol (ver el
comentario sobre `TimeSlotModel` abajo): `input.X()` solo en campos editables por un usuario;
`model.X()` en `tenant_id` (machine-supplied) y en los modelos de salida. Además, los campos de
identidad de los modelos DB llevan `NotNull: true` (el struct+tags viejo no declaraba ninguno — esa
ausencia de constraints es parte de lo que esta rectificación corrige, no un comportamiento a
portar; la doctrina fail-closed exige que el piso de validación esté declarado en la Definition).

```go
package appointmentbooking

import (
	"github.com/tinywasm/form/input"
	"github.com/tinywasm/model"
)

var EmployeeServiceConfigModel = model.Definition{
	Name: "employee_service_config",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "staff_id", Type: model.Text(), NotNull: true},
		{Name: "service_id", Type: model.Text(), NotNull: true},
		{Name: "duration_min", Type: model.Int()},
		{Name: "buffer_min", Type: model.Int()},
		{Name: "price_override", Type: model.Float()},
		{Name: "payment_required", Type: model.Bool()},
		{Name: "is_active", Type: model.Bool()},
	},
}

var WorkCalendarConfigModel = model.Definition{
	Name: "work_calendar_config",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "staff_id", Type: model.Text(), NotNull: true},
		{Name: "timezone", Type: model.Text(), NotNull: true},
		{Name: "is_active", Type: model.Bool()},
	},
}

var WorkCalendarWeeklyModel = model.Definition{
	Name: "work_calendar_weekly",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "staff_id", Type: model.Text(), NotNull: true},
		{Name: "day_of_week", Type: model.Int()},
		{Name: "work_start", Type: model.Int()},
		{Name: "work_finish", Type: model.Int()},
		{Name: "break_start", Type: model.Int()},
		{Name: "break_finish", Type: model.Int()},
		{Name: "is_active", Type: model.Bool()},
	},
}

var WorkCalendarExceptionModel = model.Definition{
	Name: "work_calendar_exception",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "staff_id", Type: model.Text(), NotNull: true},
		{Name: "specific_date", Type: model.Int()},
		{Name: "exception_type", Type: model.Text()},
		{Name: "start_time", Type: model.Int()},
		{Name: "end_time", Type: model.Int()},
		{Name: "notes", Type: model.Text()},
	},
}

// NOTA: "staff_idsnapshot" / "service_idsnapshot" preservan EXACTAMENTE el nombre de columna
// actual (irregularidad histórica, sin guión bajo) — NO renombrar la columna. Ver §1.1.
var ReservationModel = model.Definition{
	Name: "reservation",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "client_id", Type: model.Text(), NotNull: true},
		{Name: "creator_user_id", Type: model.Text()},
		{Name: "employee_service_config_id", Type: model.Text(), NotNull: true},
		{Name: "staff_idsnapshot", Type: model.Text()},
		{Name: "service_idsnapshot", Type: model.Text()},
		{Name: "duration_min_snapshot", Type: model.Int()},
		{Name: "price_snapshot", Type: model.Float()},
		{Name: "currency_snapshot", Type: model.Text()},
		{Name: "reservation_date", Type: model.Int()},
		{Name: "reservation_time", Type: model.Int()},
		{Name: "local_string_date", Type: model.Text()},
		{Name: "local_string_time", Type: model.Text()},
		// status: valores válidos = las constantes FSM exportadas de fsm.go/service.go — los
		// literales viven SOLO en esas constantes (regla anti magic-string, ver item_catalog).
		{Name: "status", Type: model.Text(), NotNull: true},
		{Name: "rescheduled_from_id", Type: model.Text()},
		{Name: "payment_id", Type: model.Text()},
		{Name: "notes", Type: model.Text()},
		{Name: "updated_at", Type: model.Int()},
		{Name: "updated_by", Type: model.Text()},
		{Name: "revision", Type: model.Int()},
	},
}

// Las 12 Definitions de abajo son transport-only. Política de widgets POR ROL (no "lo que el
// model_orm.go viejo tuviera" — ese archivo ponía widget en todo campo transport, un defecto que
// esta migración corrige): input.X() SOLO en campos que un usuario edita en un form; kinds base
// (model.X()) en campos machine-supplied (tenant_id) y en modelos de SALIDA (TimeSlot es
// resultado de list_availability — nunca se renderiza como form editable). No dejes caer un
// widget de un campo genuinamente editable: form.New() saldría vacío en silencio.

var TimeSlotModel = model.Definition{
	Name: "time_slot",
	Fields: model.Fields{
		{Name: "start_utc", Type: model.Int()},
		{Name: "end_utc", Type: model.Int()},
	},
}

var CreateReservationArgsModel = model.Definition{
	Name: "create_reservation_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "client_id", Type: input.Text()},
		{Name: "creator_user_id", Type: input.Text()},
		{Name: "employee_service_config_id", Type: input.Text()},
		{Name: "slot_start_utc", Type: input.Number()},
		{Name: "notes", Type: input.Text()},
		{Name: "rescheduled_from_id", Type: input.Text()},
	},
}

var GetReservationArgsModel = model.Definition{
	Name: "get_reservation_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "id", Type: input.Text()},
	},
}

var ListReservationsByStaffArgsModel = model.Definition{
	Name: "list_reservations_by_staff_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "staff_id", Type: input.Text()},
		{Name: "from", Type: input.Number()},
		{Name: "to", Type: input.Number()},
	},
}

var ListReservationsByClientArgsModel = model.Definition{
	Name: "list_reservations_by_client_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "client_id", Type: input.Text()},
	},
}

var ChangeReservationStatusArgsModel = model.Definition{
	Name: "change_reservation_status_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "id", Type: input.Text()},
		{Name: "event", Type: input.Text()},
		{Name: "actor_id", Type: input.Text()},
		{Name: "payment_id", Type: input.Text()},
		{Name: "revision", Type: input.Number()},
	},
}

var ExpirePendingReservationsArgsModel = model.Definition{
	Name: "expire_pending_reservations_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "before", Type: input.Number()},
	},
}

var UpsertCalendarConfigArgsModel = model.Definition{
	Name: "upsert_calendar_config_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "staff_id", Type: input.Text()},
		{Name: "timezone", Type: input.Text()},
		{Name: "is_active", Type: input.Checkbox()},
	},
}

var UpsertWeeklyCalendarArgsModel = model.Definition{
	Name: "upsert_weekly_calendar_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "staff_id", Type: input.Text()},
		{Name: "day_of_week", Type: input.Number()},
		{Name: "work_start", Type: input.Number()},
		{Name: "work_finish", Type: input.Number()},
		{Name: "break_start", Type: input.Number()},
		{Name: "break_finish", Type: input.Number()},
		{Name: "is_active", Type: input.Checkbox()},
	},
}

var AddCalendarExceptionArgsModel = model.Definition{
	Name: "add_calendar_exception_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "staff_id", Type: input.Text()},
		{Name: "specific_date", Type: input.Number()},
		{Name: "exception_type", Type: input.Text()},
		{Name: "start_time", Type: input.Number()},
		{Name: "end_time", Type: input.Number()},
		{Name: "notes", Type: input.Text()},
	},
}

var RemoveCalendarExceptionArgsModel = model.Definition{
	Name: "remove_calendar_exception_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "exception_id", Type: input.Text()},
	},
}

var ListAvailabilityArgsModel = model.Definition{
	Name: "list_availability_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()}, // machine-supplied — never a form input
		{Name: "staff_id", Type: input.Text()},
		{Name: "config_id", Type: input.Text()},
		{Name: "from", Type: input.Number()},
		{Name: "to", Type: input.Number()},
	},
}
```

### 1.4 Pasos

1. Reescribe `model.go` con el contenido de §1.3, sin directivas (`// orm:typed_fields` ya no existe).
2. `go get` las versiones de §2, luego regenera `model_orm.go` con `ormc` (instalado/actual):
   ```
   go install github.com/tinywasm/ormc/cmd/ormc@latest
   ormc   # desde la raíz del módulo
   ```
   Los 5 helpers `<Struct>_` (`EmployeeServiceConfig_`, `WorkCalendarConfig_`, `WorkCalendarWeekly_`,
   `WorkCalendarException_`, `Reservation_`) se generan automáticamente. Los 17 tipos exportados
   (5 DB + 12 args, p. ej. `CreateReservationArgs`, `GetReservationArgs`, …) también se generan a
   partir del nombre de cada `<Struct>Model` (quitando el sufijo `Model`) — estos tipos exportados los
   necesitan la Etapa 4 (`.Accepts(&CreateReservationArgs{})`) y la Etapa 5 (`ReservationList`).
3. **Casing puro (sin diccionario de acrónimos)** — afecta helper y campos de struct:
   - `tenant_id`→`TenantId`, `client_id`→`ClientId`, `payment_id`→`PaymentId`,
     `employee_service_config_id`→`EmployeeServiceConfigId`, `config_id`→`ConfigId`,
     `exception_id`→`ExceptionId`, `id`→`Id` (ya no `...ID`).
   - Columnas irregulares: `staff_idsnapshot`→`StaffIdsnapshot`, `service_idsnapshot`→
     `ServiceIdsnapshot` (un solo token tras el `_`; **no** `StaffIDSnapshot`) — verifica el nombre
     real generado y ajusta si difiere (ver §1.1).
   - Actualiza **todos** los usos en `service.go`/`repository.go` a los nuevos nombres
     (`.Where(Reservation_.Status)` no cambia; los `.XxxID`→`.XxxId` sí). Columnas/JSON del wire NO
     cambian.
4. No hay cambios de tipo `int`→`int64` en este módulo (todos los campos numéricos ya eran `int64`).

### 1.5 Fuera de alcance de esta etapa

- No renombrar columnas (incluyendo la irregularidad `staff_idsnapshot`/`service_idsnapshot`).
- No cambiar comportamiento de negocio.
- No añadir la directiva `// orm:typed_fields` (ya no existe).
- No añadir widgets a los 5 modelos DB (`EmployeeServiceConfig`…`Reservation`) — ni `input.Decimal()`
  a `price_override`/`price_snapshot`. La asignación de widgets en los transport sigue la política
  por rol de §1.3 (no el `model_orm.go` viejo).

---

## Etapa 2 — `EventPublisher` (auto-declarado) → `events.Publisher`, y fuera `tinywasm/context`

### 2.1 Contrato real de `events.Publisher` (verificado contra el código fuente del paquete)

```go
package events // github.com/tinywasm/events

type Event struct {
	Topic   string
	Payload model.Encodable // github.com/tinywasm/model — nunca un `map` o `any`
}

// Publisher es fire-and-forget: SIN valor de retorno de error.
type Publisher interface {
	Publish(e Event)
}
```

⚠️ **Nota:** el `README.md` de `veltylabs/modules` documenta `Publish(Event) error` — eso es
**inexacto** respecto al código fuente real de `github.com/tinywasm/events` a la fecha de este plan;
usa la firma verificada arriba (`Publish(e Event)`, sin error). Es una discrepancia de documentación
en un repo fuera de este módulo — no la corrijas aquí, solo úsala correctamente en este código.

### 2.2 Qué se elimina de `service.go`

```go
// ANTES — service.go líneas 3-10, 43-46
import (
	tinyctx "github.com/tinywasm/context"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	tinytime "github.com/tinywasm/time"
	"github.com/tinywasm/unixid"
)
// ...
// EventPublisher delivers domain events to other modules or infrastructure.
type EventPublisher interface {
	Publish(ctx *tinyctx.Context, event string, payload any) error
}
```

```go
// DESPUÉS
import (
	"github.com/tinywasm/events"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	tinytime "github.com/tinywasm/time"
)
// EventPublisher (auto-declarado) se elimina por completo — Deps.Publisher pasa a ser
// events.Publisher directo (§2.1). No se declara ningún tipo local en su lugar.
```

`unixid` se elimina de este archivo en la Etapa 3, no aquí — no lo quites todavía si haces las etapas
por separado; si las aplicas todas en un solo PR, el import final no lleva ni `unixid` ni `tinyctx`.

### 2.3 `ctx *tinyctx.Context` desaparece de TODA la superficie de `SchedulingService`

`ctx` no se lee en ningún método de `schedulingService` — ni `.Value()`, ni `.Method()`, ni
`.UserID()` — su ÚNICO uso en todo `service.go` es reenviarlo a `s.pub.Publish(ctx, ...)`. Como la
nueva firma de `Publish` (§2.1) ya no toma `ctx`, el parámetro queda sin ningún uso y se elimina de
la interfaz, del struct, y de cada método — exactamente como en `item_catalog.Module`, cuyos métodos
de negocio (`GetItem`, `CreateItem`, …) no toman ningún parámetro de contexto.

Quita `ctx *tinyctx.Context` como primer parámetro de **cada uno** de estos métodos (interfaz
`SchedulingService` y su implementación `schedulingService`, ambas en `service.go`):

```
UpsertCalendarConfig(ctx *tinyctx.Context, cfg WorkCalendarConfig) error
UpsertWeeklyCalendar(ctx *tinyctx.Context, cal WorkCalendarWeekly) error
AddException(ctx *tinyctx.Context, exc WorkCalendarException) error
RemoveException(ctx *tinyctx.Context, tenantID, exceptionID string) error
ListAvailability(ctx *tinyctx.Context, tenantID, staffID, configID string, from, to int64) ([]TimeSlot, error)
CreateReservation(ctx *tinyctx.Context, cmd CreateReservationCmd) (Reservation, error)
GetReservation(ctx *tinyctx.Context, tenantID, id string) (Reservation, error)
ListReservationsByStaff(ctx *tinyctx.Context, tenantID, staffID string, from, to int64) ([]Reservation, error)
ListReservationsByClient(ctx *tinyctx.Context, tenantID, clientID string) ([]Reservation, error)
ChangeReservationStatus(ctx *tinyctx.Context, cmd ChangeStatusCmd) error
ExpirePendingReservations(ctx *tinyctx.Context, tenantID string, before int64) (int, error)
```

pasa a:

```
UpsertCalendarConfig(cfg WorkCalendarConfig) error
UpsertWeeklyCalendar(cal WorkCalendarWeekly) error
AddException(exc WorkCalendarException) error
RemoveException(tenantID, exceptionID string) error
ListAvailability(tenantID, staffID, configID string, from, to int64) ([]TimeSlot, error)
CreateReservation(cmd CreateReservationCmd) (Reservation, error)
GetReservation(tenantID, id string) (Reservation, error)
ListReservationsByStaff(tenantID, staffID string, from, to int64) ([]Reservation, error)
ListReservationsByClient(tenantID, clientID string) ([]Reservation, error)
ChangeReservationStatus(cmd ChangeStatusCmd) error
ExpirePendingReservations(tenantID string, before int64) (int, error)
```

Dentro de cada implementación, elimina también las llamadas internas que reenviaban `ctx` (p. ej.
`s.ListAvailability(ctx, cmd.TenantID, ...)` dentro de `CreateReservation` pasa a
`s.ListAvailability(cmd.TenantID, ...)`).

### 2.4 `Deps`/`Module` — renombra `schedulingService` → `Module`

```go
// ANTES
type Deps struct {
	Staff     StaffReader
	Catalog   CatalogReader
	Directory DirectoryReader
	Publisher EventPublisher
}

type schedulingService struct {
	db        *orm.DB
	repo      *Repository
	staff     StaffReader
	catalog   CatalogReader
	directory DirectoryReader
	pub       EventPublisher
}

func New(db *orm.DB, deps Deps) (SchedulingService, error) {
	repo, err := NewRepository(db)
	if err != nil {
		return nil, err
	}
	return &schedulingService{
		db: db, repo: repo,
		staff: deps.Staff, catalog: deps.Catalog, directory: deps.Directory,
		pub: deps.Publisher,
	}, nil
}
```

```go
// DESPUÉS — Deps gana IDs (Etapa 3); New retorna *Module (concreto, no la interfaz) para que
// Module pueda implementar router.OpModule (Etapa 4) y NewView (Etapa 5) directamente, exactamente
// como itemcatalog.New retorna *itemcatalog.Module. SchedulingService se conserva como
// documentación del contrato de negocio + verificación en tiempo de compilación.
type Deps struct {
	Staff     StaffReader
	Catalog   CatalogReader
	Directory DirectoryReader
	IDs       model.IDGenerator // requerido — Etapa 3
	Publisher events.Publisher  // opcional — nil desactiva la publicación de eventos
}

type Module struct {
	db        *orm.DB
	repo      *Repository
	ids       model.IDGenerator
	staff     StaffReader
	catalog   CatalogReader
	directory DirectoryReader
	pub       events.Publisher
}

func New(db *orm.DB, deps Deps) (*Module, error) {
	if deps.IDs == nil {
		return nil, fmt.Err("appointment_booking: Deps.IDs is required")
	}
	repo, err := NewRepository(db, deps.IDs) // firma nueva — ver Etapa 3
	if err != nil {
		return nil, err
	}
	return &Module{
		db: db, repo: repo, ids: deps.IDs,
		staff: deps.Staff, catalog: deps.Catalog, directory: deps.Directory,
		pub: deps.Publisher,
	}, nil
}

var _ SchedulingService = (*Module)(nil) // el contrato de negocio sigue documentado y verificado
```

Cambia todos los métodos de `func (s *schedulingService) X(...)` a `func (m *Module) X(...)` (renombra
también el receptor de `s` a `m` por consistencia con el resto del ecosistema — no es obligatorio para
compilar, pero mantenlo consistente en todo el archivo).

### 2.5 Cada llamada a `s.pub.Publish(ctx, topic, payload)` pasa a `m.pub.Publish(events.Event{...})`

Hay 3 call sites en `service.go`:

```go
// ANTES (dentro de CreateReservation)
if s.pub != nil {
	s.pub.Publish(ctx, EventReservationCreated, newReservation)
	if originalReservation != nil {
		s.pub.Publish(ctx, EventReservationRescheduled, *originalReservation)
	}
}
```
```go
// DESPUÉS
if m.pub != nil {
	m.pub.Publish(events.Event{Topic: EventReservationCreated, Payload: &newReservation})
	if originalReservation != nil {
		m.pub.Publish(events.Event{Topic: EventReservationRescheduled, Payload: originalReservation})
	}
}
```

```go
// ANTES (dentro de ChangeReservationStatus)
if s.pub != nil && domainEvent != "" {
	updated, _ := s.repo.GetReservation(cmd.ID)
	s.pub.Publish(ctx, domainEvent, updated)
}
```
```go
// DESPUÉS
if m.pub != nil && domainEvent != "" {
	updated, _ := m.repo.GetReservation(cmd.ID)
	m.pub.Publish(events.Event{Topic: domainEvent, Payload: &updated})
}
```

`Reservation` ya implementa `model.Encodable` (generado por `ormc` en la Etapa 1) — pasar `&updated`/
`&newReservation`/`originalReservation` (ya es puntero) satisface `Payload model.Encodable` sin
cambios adicionales.

### 2.6 Criterio de aceptación de esta etapa

```
grep -rn "tinywasm/context" service.go repository.go mcp.go ops.go 2>/dev/null   # vacío (no-test)
grep -rn "EventPublisher" service.go                                             # vacío
grep -n "ctx \*tinyctx.Context\|ctx *context.Context" service.go                # vacío
```

---

## Etapa 3 — `unixid.NewUnixID()` → `Deps.IDs model.IDGenerator`

### 3.1 `repository.go` — `NewRepository` recibe el generador

```go
// ANTES
import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

type Repository struct {
	db *orm.DB
}

func NewRepository(db *orm.DB) (*Repository, error) {
	r := &Repository{db: db}
	tables := []fmt.Model{
		&EmployeeServiceConfig{}, &WorkCalendarConfig{}, &WorkCalendarWeekly{},
		&WorkCalendarException{}, &Reservation{},
	}
	for _, t := range tables {
		if err := db.CreateTable(t); err != nil {
			return nil, err
		}
	}
	return r, nil
}
```

```go
// DESPUÉS — la migración de schema se mueve a ddl.CreateTable en la Etapa 6; aquí solo se
// muestra el cambio de generador de ID. Ver Etapa 6 para el NewRepository final completo.
import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
)

type Repository struct {
	db  *orm.DB
	ids model.IDGenerator
}

func NewRepository(db *orm.DB, ids model.IDGenerator) (*Repository, error) {
	r := &Repository{db: db, ids: ids}
	// ... migración de schema — ver Etapa 6, no dupliques esta lógica aquí
	return r, nil
}
```

### 3.2 Cada `unixid.NewUnixID()` inline en `repository.go` pasa a `r.ids.NewID()`

5 call sites, todos con el mismo patrón mecánico. Ejemplo (`InsertReservation`):

```go
// ANTES
func (r *Repository) InsertReservation(res *Reservation) error {
	idHandler, err := unixid.NewUnixID()
	if err != nil {
		return err
	}
	if res.ID == "" {
		res.ID = idHandler.NewID()
	}
	res.Revision = 0
	return r.db.Create(res)
}
```
```go
// DESPUÉS
func (r *Repository) InsertReservation(res *Reservation) error {
	if res.Id == "" { // casing: Id, no ID — ver Etapa 1 §1.4
		res.Id = r.ids.NewID()
	}
	res.Revision = 0
	return r.db.Create(res)
}
```

Aplica el mismo cambio mecánico (quitar `idHandler, err := unixid.NewUnixID()` + chequeo de error,
usar `r.ids.NewID()` directo — `model.IDGenerator.NewID()` no retorna error) en:
- `InsertException` (`exc.ID = idHandler.NewID()` → `exc.Id = r.ids.NewID()`)
- `InsertEmployeeServiceConfig` (`cfg.ID = idHandler.NewID()` → `cfg.Id = r.ids.NewID()`)
- `UpsertCalendarConfig` (rama `err == orm.ErrNotFound`: `cfg.ID = idHandler.NewID()` →
  `cfg.Id = r.ids.NewID()`)
- `UpsertWeeklyCalendar` (rama `err == orm.ErrNotFound`: `cal.ID = idHandler.NewID()` →
  `cal.Id = r.ids.NewID()`)

### 3.3 `service.go` — `CreateReservation`'s inline `unixid.NewUnixID()`

```go
// ANTES (dentro de la Tx de CreateReservation)
idHandler, err := unixid.NewUnixID()
if err != nil {
	return err
}
// ensure a unique ID by using the generated nanosecond one properly
newReservation.ID = idHandler.NewID()
```
```go
// DESPUÉS
newReservation.Id = m.ids.NewID() // casing: Id — ver Etapa 1 §1.4; m.ids viene de Deps.IDs (Etapa 2 §2.4)
```

### 3.4 Criterio de aceptación

```
grep -rn "tinywasm/unixid" .   # vacío, todo el repo, tests incluidos
```

---

## Etapa 4 — `mcp.ToolProvider` → `router.OpModule` (un solo `OpModule`, 11 ops)

### 4.1 Decisión: un `OpModule`, no dos

`item_catalog` (la referencia validada) implementa **un** `router.OpModule` para 11 ops que cubren
dos sub-dominios (catálogo + convenios) sin partir por sub-dominio — `ModelName()` retorna un solo
nombre y `MountOps` registra las 11. Este módulo tiene exactamente el mismo conteo: 6 ops de
reservación + 5 ops de calendario = 11. No hay ninguna razón específica de este dominio (RBAC
distinto, ciclo de vida distinto, transporte distinto) que justifique dos `OpModule` — los nombres de
recurso (`"reservation"` vs `"calendar"`) ya distinguen los dos sub-dominios dentro de las mismas 11
`.Requires(resource, action)`, igual que `item_catalog` distingue `"catalog_item"` de
`"catalog_agreement"` dentro de su único `OpModule`. Un solo `*Module` implementa `router.OpModule`.

### 4.2 Renombra `mcp.go` → `ops.go`

A diferencia de `item_catalog` (cuyo `mcp.go` mezclaba lógica de negocio y transporte, por lo que
renombrarlo habría sido puro churn), el `mcp.go` de este repo **ya es** puramente transporte
(`ReservationProvider`/`CalendarProvider`, delegando todo a `SchedulingService`) — mapea 1:1 al
archivo `ops.go` de la convención de estructura de archivos (`README.md` de `veltylabs/modules`).
`git mv mcp.go ops.go`.

### 4.3 Contrato real usado (verificado contra el código fuente de `tinywasm/router`)

```go
// github.com/tinywasm/router
type OpRegistry interface {
	Op(name string, h HandlerFunc) Route
}
type OpModule interface {
	model.ModuleNaming // ModelName() string
	MountOps(reg OpRegistry)
}
type Route interface {
	Requires(resource model.Resource, action model.Action) Route
	Accepts(args model.Fielder) Route
	// ...
}
type Context interface {
	// ...
	Decode(into model.Decodable) error
	Encode(v model.Encodable) error
	WriteStatus(code int)
	Write(b []byte) (int, error)
}
```
`model.Action` es un bitmask con constantes `model.Create`/`model.Read`/`model.Update`/`model.Delete`
(no los bytes `'c'/'r'/'u'/'d'` de `mcp.Tool.Action` — ese tipo desaparece con `mcp.go`).

### 4.4 Contenido completo de `ops.go`

```go
package appointmentbooking

import "github.com/tinywasm/router"

const (
	OpCreateReservation          = "create_reservation"
	OpGetReservation             = "get_reservation"
	OpListReservationsByStaff    = "list_reservations_by_staff"
	OpListReservationsByClient   = "list_reservations_by_client"
	OpChangeReservationStatus    = "change_reservation_status"
	OpExpirePendingReservations  = "expire_pending_reservations"
	OpUpsertCalendarConfig       = "upsert_calendar_config"
	OpUpsertWeeklyCalendar       = "upsert_weekly_calendar"
	OpAddCalendarException       = "add_calendar_exception"
	OpRemoveCalendarException    = "remove_calendar_exception"
	OpListAvailability           = "list_availability"
)

func (m *Module) ModelName() string { return "appointment_booking" }

func (m *Module) MountOps(reg router.OpRegistry) {
	reg.Op(OpCreateReservation, m.opCreateReservation).Requires("reservation", model.Create).Accepts(&CreateReservationArgs{})
	reg.Op(OpGetReservation, m.opGetReservation).Requires("reservation", model.Read).Accepts(&GetReservationArgs{})
	reg.Op(OpListReservationsByStaff, m.opListReservationsByStaff).Requires("reservation", model.Read).Accepts(&ListReservationsByStaffArgs{})
	reg.Op(OpListReservationsByClient, m.opListReservationsByClient).Requires("reservation", model.Read).Accepts(&ListReservationsByClientArgs{})
	reg.Op(OpChangeReservationStatus, m.opChangeReservationStatus).Requires("reservation", model.Update).Accepts(&ChangeReservationStatusArgs{})
	reg.Op(OpExpirePendingReservations, m.opExpirePendingReservations).Requires("reservation", model.Update).Accepts(&ExpirePendingReservationsArgs{})
	// Upserts: crean en la rama not-found Y actualizan en la otra — el op exige TODAS las
	// acciones que realmente puede ejecutar (model.Action es bitmask). Declarar solo Update
	// dejaría a un principal update-only creando filas (violación de closed-by-default).
	reg.Op(OpUpsertCalendarConfig, m.opUpsertCalendarConfig).Requires("calendar", model.Create|model.Update).Accepts(&UpsertCalendarConfigArgs{})
	reg.Op(OpUpsertWeeklyCalendar, m.opUpsertWeeklyCalendar).Requires("calendar", model.Create|model.Update).Accepts(&UpsertWeeklyCalendarArgs{})
	reg.Op(OpAddCalendarException, m.opAddCalendarException).Requires("calendar", model.Create).Accepts(&AddCalendarExceptionArgs{})
	reg.Op(OpRemoveCalendarException, m.opRemoveCalendarException).Requires("calendar", model.Delete).Accepts(&RemoveCalendarExceptionArgs{})
	reg.Op(OpListAvailability, m.opListAvailability).Requires("calendar", model.Read).Accepts(&ListAvailabilityArgs{})
}

var _ router.OpModule = (*Module)(nil)

// writeError maps known sentinel errors to an HTTP-ish status code and writes err.Error() as the
// body, preserving (loosely) the human-readable messages the old mcp.Result{Content: msg} gave —
// router.Context has no error-with-message envelope of its own, so this is the module's own,
// minimal convention. See docs/PLAN.md §4 "Fuera de alcance" for why this isn't richer.
//
// Convención de mapeo (la misma para todos los módulos del ecosistema — nunca colapsar todo a
// 500, eso es el "runtime mystery" que CONSTRUCTION_HARNESS prohíbe):
//   400 = decode/validación/precondición inválida · 404 = no existe · 409 = conflicto · 500 = resto.
func writeError(ctx router.Context, err error) {
	switch err {
	case ErrNotFound:
		ctx.WriteStatus(404)
	case ErrSlotTaken, ErrConflict:
		ctx.WriteStatus(409)
	case ErrCalendarConfigNotFound, ErrInvalidTransition:
		ctx.WriteStatus(400)
	default:
		ctx.WriteStatus(500)
	}
	ctx.Write([]byte(err.Error()))
}

func (m *Module) opCreateReservation(ctx router.Context) {
	var args CreateReservationArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	// Doctrina fail-closed: decode → validate → servicio. Validate ejecuta las constraints
	// declaradas en la Definition (método generado por ormc — nunca re-implementado a mano).
	// Aplica este mismo patrón en los 11 handlers: todo op que decodifica args valida antes
	// de llamar al método de negocio; error de validación ⇒ 400.
	if err := args.Validate(model.ActionCreate); err != nil {
		ctx.WriteStatus(400)
		return
	}
	cmd := CreateReservationCmd{
		TenantID:                args.TenantId,
		ClientID:                args.ClientId,
		CreatorUserID:           args.CreatorUserId,
		EmployeeServiceConfigID: args.EmployeeServiceConfigId,
		SlotStartUTC:            args.SlotStartUtc,
		Notes:                   args.Notes,
		RescheduledFromID:       args.RescheduledFromId,
	}
	res, err := m.CreateReservation(cmd)
	if err != nil {
		writeError(ctx, err)
		return
	}
	if err := ctx.Encode(&res); err != nil {
		ctx.WriteStatus(500)
	}
}

func (m *Module) opGetReservation(ctx router.Context) {
	var args GetReservationArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	res, err := m.GetReservation(args.TenantId, args.Id)
	if err != nil {
		writeError(ctx, err)
		return
	}
	if err := ctx.Encode(&res); err != nil {
		ctx.WriteStatus(500)
	}
}

func (m *Module) opListReservationsByStaff(ctx router.Context) {
	var args ListReservationsByStaffArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	res, err := m.ListReservationsByStaff(args.TenantId, args.StaffId, args.From, args.To)
	if err != nil {
		writeError(ctx, err)
		return
	}
	list := make(ReservationList, len(res))
	for i := range res {
		list[i] = &res[i]
	}
	if err := ctx.Encode(&list); err != nil {
		ctx.WriteStatus(500)
	}
}

func (m *Module) opListReservationsByClient(ctx router.Context) {
	var args ListReservationsByClientArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	res, err := m.ListReservationsByClient(args.TenantId, args.ClientId)
	if err != nil {
		writeError(ctx, err)
		return
	}
	list := make(ReservationList, len(res))
	for i := range res {
		list[i] = &res[i]
	}
	if err := ctx.Encode(&list); err != nil {
		ctx.WriteStatus(500)
	}
}

func (m *Module) opChangeReservationStatus(ctx router.Context) {
	var args ChangeReservationStatusArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	cmd := ChangeStatusCmd{
		TenantID:  args.TenantId,
		ID:        args.Id,
		Event:     args.Event,
		ActorID:   args.ActorId,
		PaymentID: args.PaymentId,
		Revision:  int(args.Revision),
	}
	if err := m.ChangeReservationStatus(cmd); err != nil {
		writeError(ctx, err)
		return
	}
	ctx.WriteStatus(200)
}

func (m *Module) opExpirePendingReservations(ctx router.Context) {
	var args ExpirePendingReservationsArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	count, err := m.ExpirePendingReservations(args.TenantId, args.Before)
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.Write([]byte(fmt.Convert(count).String()))
}

func (m *Module) opUpsertCalendarConfig(ctx router.Context) {
	var args UpsertCalendarConfigArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	cfg := WorkCalendarConfig{
		TenantId: args.TenantId, StaffId: args.StaffId,
		Timezone: args.Timezone, IsActive: args.IsActive,
	}
	if err := m.UpsertCalendarConfig(cfg); err != nil {
		writeError(ctx, err)
		return
	}
	ctx.WriteStatus(200)
}

func (m *Module) opUpsertWeeklyCalendar(ctx router.Context) {
	var args UpsertWeeklyCalendarArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	cal := WorkCalendarWeekly{
		TenantId: args.TenantId, StaffId: args.StaffId, DayOfWeek: args.DayOfWeek,
		WorkStart: args.WorkStart, WorkFinish: args.WorkFinish,
		BreakStart: args.BreakStart, BreakFinish: args.BreakFinish, IsActive: args.IsActive,
	}
	if err := m.UpsertWeeklyCalendar(cal); err != nil {
		writeError(ctx, err)
		return
	}
	ctx.WriteStatus(200)
}

func (m *Module) opAddCalendarException(ctx router.Context) {
	var args AddCalendarExceptionArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	exc := WorkCalendarException{
		TenantId: args.TenantId, StaffId: args.StaffId, SpecificDate: args.SpecificDate,
		ExceptionType: args.ExceptionType, StartTime: args.StartTime, EndTime: args.EndTime,
		Notes: args.Notes,
	}
	if err := m.AddException(exc); err != nil {
		writeError(ctx, err)
		return
	}
	ctx.WriteStatus(200)
}

func (m *Module) opRemoveCalendarException(ctx router.Context) {
	var args RemoveCalendarExceptionArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := m.RemoveException(args.TenantId, args.ExceptionId); err != nil {
		writeError(ctx, err)
		return
	}
	ctx.WriteStatus(200)
}

func (m *Module) opListAvailability(ctx router.Context) {
	var args ListAvailabilityArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	slots, err := m.ListAvailability(args.TenantId, args.StaffId, args.ConfigId, args.From, args.To)
	if err != nil {
		writeError(ctx, err)
		return
	}
	list := make(TimeSlotList, len(slots))
	for i := range slots {
		list[i] = &slots[i]
	}
	if err := ctx.Encode(&list); err != nil {
		ctx.WriteStatus(500)
	}
}
```

Añade `"github.com/tinywasm/fmt"` y `"github.com/tinywasm/model"` al bloque de imports de `ops.go`
(usados por `fmt.Convert(...)` en `opExpirePendingReservations` y por `model.Create`/`model.Read`/
`model.Update`/`model.Delete` en `MountOps`).

**Nota de campo exacto:** los nombres `args.TenantId`/`args.ClientId`/`args.SlotStartUtc`/etc. de
arriba asumen el casing puro que resulte de la Etapa 1 (`SlotStartUTC`→`SlotStartUtc` es la regla
"un solo token tras mayúscula inicial, sin diccionario de acrónimos" — verifica el nombre real que
`ormc` genera para `slot_start_utc` tras la Etapa 1 y ajusta si difiere; no asumas que este plan
adivinó cada sigla correctamente).

### 4.5 Elimina el `EventPublisher`/`mcp` residual

Borra por completo `ReservationProvider`, `CalendarProvider`, `NewReservationProvider`,
`NewCalendarProvider`, `errResult`, y los imports `"github.com/tinywasm/context"`,
`"github.com/tinywasm/json"`, `"github.com/tinywasm/mcp"` — nada de esto sobrevive en `ops.go`.

### 4.6 Criterio de aceptación

```
grep -rn "tinywasm/mcp\|tinywasm/json" .                          # vacío, todo el repo
grep -n "mcp.ToolProvider\|mcp.Tool\b" ops.go                      # vacío
grep -n "var _ router.OpModule" ops.go                             # presente
```

---

## Etapa 5 — `NewView(caller router.Caller, tenantId, staffId string) view.Presenter`

### 5.1 Decisión de diseño — una sola vista, para `Reservation`, con dos parámetros extra

Sigue el patrón exacto de `item_catalog.NewView` (`view.New` con 5 argumentos posicionales +
`opts...`, incluyendo `view.WithFill` — la firma que corresponde a `github.com/tinywasm/view@v0.1.1`,
la versión objetivo de este plan; ver §2). Dos decisiones explícitas:

1. **Solo `Reservation` tiene vista — no hay una segunda vista para calendarios.**
   `WorkCalendarConfig`/`WorkCalendarWeekly`/`WorkCalendarException` son datos de configuración de
   baja cardinalidad (1 fila, ≤7 filas, un puñado de excepciones por staff) sin un "list op" natural
   que un usuario navegue como una lista — los `upsert_*`/`add_*`/`remove_*` ya cubren cada mutación.
   Construir un segundo `view.Presenter` sería fabricar una UI de lista para datos que no tienen forma
   de lista. Documentado explícitamente en `docs/ARCHITECTURE.md` §7 — no es un vacío de este plan.
2. **`NewView` necesita `tenantId, staffId string` además de `caller`** — no hay ninguna operación
   "listar todas las reservaciones del tenant" sin filtro; la única op de listado con sentido para una
   vista de agenda es `list_reservations_by_staff`, que requiere `staffId`. Esto es una desviación
   deliberada de la firma `NewView(caller router.Caller) view.Presenter` que módulos más simples (como
   `item_catalog`) usan sin parámetros extra — documentada en `docs/ARCHITECTURE.md` §7. Nada en
   `AGENTS.md` exige que `NewView` tome exactamente un parámetro; exige que se construya solo con
   `view`+`model`+`router`, lo cual se cumple.
3. **Sin `Saver`/`Deleter`**: las reservaciones nunca se editan como registro completo (mutan solo vía
   `ChangeReservationStatus`, gated por FSM) y nunca se borran físicamente — así que
   `view.WithSaveOp`/`view.WithDeleteOp` se omiten a propósito. Un `Presenter` desnudo (list+select) es
   la forma correcta aquí, no un vacío.

### 5.2 Contenido de `view.go` (archivo nuevo)

```go
package appointmentbooking

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// NewView builds the Reservation Presenter — scoped to one staff member's schedule, since there is
// no unscoped "list all reservations for a tenant" op (see docs/ARCHITECTURE.md §7). List-only: no
// Saver/Deleter capability (reservations mutate only via ChangeReservationStatus's FSM-gated
// transitions, and are never hard-deleted).
func NewView(caller router.Caller, tenantId, staffId string) view.Presenter {
	// Cache privado como SLICE con scan lineal — NO map: la regla "cero map" de AGENTS.md no
	// tiene excepciones (ni siquiera estado privado; el runtime de map de TinyGo viaja en el
	// binario wasm igual). Mismo patrón que item_catalog/view.go tras su review.
	var byId []*Reservation
	record := &Reservation{}

	return view.New(
		caller,
		record,
		OpListReservationsByStaff,
		func() model.FielderSlice { return &ReservationList{} },
		func(list model.FielderSlice) []view.Item {
			l := list.(*ReservationList)
			items := make([]view.Item, l.Len())
			byId = make([]*Reservation, l.Len())
			for i := 0; i < l.Len(); i++ {
				it := l.At(i).(*Reservation)
				byId[i] = it
				items[i] = view.Item{
					ID:          it.Id,
					Label:       it.LocalStringDate + " " + it.LocalStringTime,
					Description: it.Status,
				}
			}
			return items
		},
		view.WithTitle("Reservas"),
		view.WithArgs(func() model.Encodable {
			return &ListReservationsByStaffArgs{TenantId: tenantId, StaffId: staffId}
		}),
		view.WithFill(func(id string) model.Model {
			if id == "" {
				return nil
			}
			for _, it := range byId {
				if it != nil && it.Id == id {
					return it
				}
			}
			return nil
		}),
	)
}
```

`NewView` es un método libre (no de `*Module`) igual que `item_catalog`'s no lo es tampoco — es una
función a nivel de paquete, no depende de estado del `*Module` más allá de `caller`.

### 5.3 Criterio de aceptación

```
grep -n "func NewView" view.go   # presente, firma exacta de §5.2
go vet ./...                     # limpio
```

---

## Etapa 6 — Migración de schema vía `ddl.CreateTable` (5 entidades propias)

Este módulo **posee** su esquema (a diferencia de `work_schedule`, que solo lee tablas ajenas) — migra
las 5 tablas DB en `New()`/`NewRepository`, igual que hace hoy con `db.CreateTable`, pero a través de
`github.com/tinywasm/ddl` con el idiom de type-assertion (`ddl.Compiler` es una capacidad opcional que
solo implementan backends SQL; `storage/mem`, usado por los tests de este módulo, no la implementa y
el bloque entero se vuelve un no-op contra ese backend).

### 6.1 `repository.go` — `NewRepository` final (reemplaza íntegro el de la Etapa 3 §3.1)

```go
package appointmentbooking

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
)

// Repository provides CRUD operations for all appointment-booking tables.
type Repository struct {
	db  *orm.DB
	ids model.IDGenerator
}

// NewRepository creates a new Repository and migrates its 5 owned tables when the backend
// supports DDL (a no-op against storage/mem, used by this module's own tests).
func NewRepository(db *orm.DB, ids model.IDGenerator) (*Repository, error) {
	tables := []model.Model{
		&EmployeeServiceConfig{},
		&WorkCalendarConfig{},
		&WorkCalendarWeekly{},
		&WorkCalendarException{},
		&Reservation{},
	}
	if ddlCompiler, ok := db.RawConn().(ddl.Compiler); ok {
		for _, t := range tables {
			if err := ddl.New(db.RawConn(), ddlCompiler).CreateTable(t); err != nil {
				return nil, err
			}
		}
	}
	return &Repository{db: db, ids: ids}, nil
}
```

`db.CreateTable(t)` (la llamada vieja, ya removida de `*orm.DB` en las versiones a las que este plan
apunta — ver §2) desaparece por completo; `fmt.Model` (el alias usado por el `model_orm.go` viejo)
también desaparece — la Etapa 1 ya regenera `model_orm.go` sobre `model.Model`.

### 6.2 Criterio de aceptación

```
grep -n "db.CreateTable\|fmt.Model" repository.go   # vacío
grep -n "tinywasm/ddl" go.mod                        # presente, dependencia directa
```

---

## Etapa 7 — Tests → `tests/`, `orm.New(mem.New())`, sin `tinywasm/sqlite` en ningún lado

### 7.1 Qué cambia y dónde (archivo por archivo, todos ya en `tests/`)

| Archivo | Cambio |
|---|---|
| `tests/repository_test.go` | `newTestRepo` abre `sqlite.Open(":memory:")` → `orm.New(mem.New())`; `ab.NewRepository(db)` → `ab.NewRepository(db, &fakeIDs{})` (ver §7.3) |
| `tests/service_back_test.go` | mismo cambio de `db`; quita el `//go:build !wasm` (ya no hace falta — `storage/mem` es isomorfo, a diferencia de `tinywasm/sqlite`, que requería CGO — ver §7.4); quita todos los `ctx :=`/`ctx,`/`ctx)` de las llamadas a `svc.*` (Etapa 2 §2.3 quitó `ctx` de `SchedulingService`) |
| `tests/service_front_test.go` | se **elimina** — era el stub `//go:build wasm` que existía solo porque `tinywasm/sqlite` bloqueaba wasm; con `storage/mem` ya no hace falta ninguna partición por build tag (§7.4) |
| `tests/setup_test.go` | quita todos los `ctx :=`/pasos de `ctx` a `s.*`; `tinyctx "github.com/tinywasm/context"` deja de importarse |
| `tests/service_runner_test.go` | igual (quita `ctx`, quita import `tinyctx`); además quita el import stdlib `"strings"` (prohibido por `AGENTS.md`) — usa `github.com/tinywasm/fmt` si necesitas alguna operación de substring, o revisa si el uso de `strings` en ese archivo es simplemente innecesario tras el resto de los cambios y elimínalo |
| `tests/availability_runner_test.go` | igual (quita `ctx`, quita import `tinyctx`) |
| `tests/mocks_test.go` | `MockEventPublisher.Publish` cambia de firma (§7.2); `MockStaffReader`/`MockCatalogReader`/`MockDirectoryReader` no cambian (implementan `StaffReader`/`CatalogReader`/`DirectoryReader`, que no son parte de esta migración); añade `fakeIDs` (§7.3) |
| `tests/mcp_test.go` | se **elimina** por completo — testeaba `mcp.ToolProvider`, que ya no existe; su cobertura de "mensaje de error legible" (`upsert_weekly_calendar_no_config`, `create_reservation_slot_taken`) se reemplaza por un test nuevo contra `router/mock` (§7.5) |
| `tests/fsm_test.go` | sin cambios — no toca infraestructura |

### 7.2 `MockEventPublisher` — nueva firma

```go
// ANTES
type MockEventPublisher struct {
	PublishedEvents []string
	Err             error
}

func (m *MockEventPublisher) Publish(ctx *tinyctx.Context, event string, payload any) error {
	m.PublishedEvents = append(m.PublishedEvents, event)
	return m.Err
}
```
```go
// DESPUÉS — Err desaparece: events.Publisher.Publish no retorna error (§2.1)
import "github.com/tinywasm/events"

type MockEventPublisher struct {
	PublishedEvents []string
}

func (m *MockEventPublisher) Publish(e events.Event) {
	m.PublishedEvents = append(m.PublishedEvents, e.Topic)
}

var _ events.Publisher = (*MockEventPublisher)(nil)
```
El resto del repo que hace `deps.Publisher.(*MockEventPublisher)` y lee `.PublishedEvents` (p. ej.
`tests/service_back_test.go`) no necesita cambios adicionales — la forma de la lista de topics
publicados (`[]string`) se conserva.

### 7.3 `fakeIDs` — nuevo test double para `model.IDGenerator`

```go
// tests/mocks_test.go — añadir
type fakeIDs struct{ n int }

func (f *fakeIDs) NewID() string {
	f.n++
	return "test-id-" + fmt.Convert(f.n).String() // github.com/tinywasm/fmt — nunca strconv
}
```
`SetupDependencies()` (en `tests/mocks_test.go`) gana `IDs: &fakeIDs{}` en el `ab.Deps{}` que
construye.

### 7.4 `storage/mem` es isomorfo — colapsa la partición `!wasm`/`wasm`

`tinywasm/sqlite` requiere CGO (bindings nativos) y por eso bloqueaba compilar a wasm — de ahí el
`//go:build !wasm` en varios archivos y el stub vacío `//go:build wasm` en
`tests/service_front_test.go`. `github.com/tinywasm/storage/mem` es Go puro, compila igual bajo
`GOOS=js GOARCH=wasm`/TinyGo que bajo el target nativo — no hace falta ninguna partición por build
tag para la construcción de `*orm.DB`. Quita `//go:build !wasm` de `tests/service_back_test.go`,
`tests/repository_test.go`, y `tests/mcp_test.go` (este último se elimina, ver arriba) — y borra
`tests/service_front_test.go` entero (el estaba ahí únicamente para "cubrir" el hueco que dejaba
`!wasm` en el resto).

### 7.5 `router/mock` — reemplazo de la cobertura de `tests/mcp_test.go`

Nuevo archivo `tests/ops_test.go`, drive `MountOps` contra `router/mock` (satisface
`router.OpRegistry`) en vez de contra `mcp.ToolProvider`:

```go
package tests

import (
	"testing"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/storage/mem"
	ab "github.com/veltylabs/appointment_booking"
)

func TestMountOps_CreateReservation_SlotTaken(t *testing.T) {
	db := orm.New(mem.New())
	m, err := ab.New(db, SetupDependencies())
	if err != nil {
		t.Fatalf("ab.New: %v", err)
	}

	// API real de router/mock — la misma que item_catalog/tests usa en verde (no adivinar):
	reg := &mock.Router{}
	reg.Configure(mock.Config{
		Authorize: func(userID string, r model.Resource, a model.Action) bool { return true },
	})
	m.MountOps(reg)

	// Sembrar (reutiliza los helpers de seeding que tests/service_back_test.go ya usa: config de
	// empleado-servicio + calendario del staff), luego invocar create_reservation dos veces con el
	// mismo slot; la segunda debe responder 409 (ErrSlotTaken via writeError):
	body := []byte(`{"tenant_id":"t1","client_id":"c1","creator_user_id":"u1",` +
		`"employee_service_config_id":"cfg1","slot_start_utc":1700000000}`)

	ok := &mock.Context{InBody: body}
	ok.SetUserID("u1")
	reg.Invoke("OP", "/"+ab.OpCreateReservation, ok)
	if ok.Status != 0 && ok.Status != 200 {
		t.Fatalf("primera reserva: status %d, body=%s", ok.Status, ok.ResponseBody())
	}

	taken := &mock.Context{InBody: body}
	taken.SetUserID("u1")
	reg.Invoke("OP", "/"+ab.OpCreateReservation, taken)
	if taken.Status != 409 {
		t.Fatalf("slot tomado: se esperaba 409, got %d", taken.Status)
	}
}
```
(Imports adicionales del archivo: `"github.com/tinywasm/model"`.) Añade también el caso de
denegación RBAC: un `&mock.Router{}` SIN `Authorize` configurado deniega toda ruta guardada —
invoca cualquier op y asserta `ctx.Status == 403`. El resto de la cobertura de
FSM/disponibilidad/repositorio no necesita un test nuevo: ya existe en `tests/fsm_test.go`,
`tests/availability_runner_test.go`, `tests/service_runner_test.go`.

### 7.5b `tests/tenant_isolation_test.go` — NUEVO, obligatorio (no va al backlog)

El review de `item_catalog` (PR #5) encontró dos bugs reales de cross-tenant (update/delete
condicionados solo por `Id`). Esta clase de bug se previene con dos reglas que este plan hace
explícitas:

1. **Toda condición de UPDATE/DELETE en `repository.go`/`service.go` incluye `TenantId`**, no solo
   el pre-read (`GetReservation` y luego update por `Id` es una ventana TOCTOU; la condición misma
   lleva tenant). Revisa cada `.Where(...)`/`orm.Eq(...)` de escritura y añade la columna tenant si
   falta.
2. **Tests de aislamiento en este plan** (no en `PLAN_TESTS_BACKUP.md`): nuevo archivo
   `tests/tenant_isolation_test.go` con, como mínimo — tenant B no puede `GetReservation` una
   reserva de tenant A (not-found); `ChangeReservationStatus` con `TenantID` de B sobre una reserva
   de A falla sin mutarla; `RemoveException` de B sobre una excepción de A falla y la fila
   sobrevive. Mismo `setup` que el resto de la suite, dos tenants sembrados.

### 7.5c `tests/conformance_test.go` — la vista se prueba con `view/conformance`

Por convención del ecosistema (AGENTS.md, Testing) el suite de conformance va en su **propio
archivo** `tests/conformance_test.go`: ejercita el `view.Presenter` de la Etapa 5 con el
`FakeCaller` de `github.com/tinywasm/view/conformance` (list → items proyectados con
`Label = LocalStringDate + " " + LocalStringTime`, select vía `WithFill`, y asserta que NO hay
capacidad save/delete — el Presenter es list+select por diseño, §5.1). Copia la forma exacta de
`item_catalog/tests/conformance_test.go` (referencia en verde) adaptando ops/campos.

### 7.6 Criterio de aceptación

```
grep -rn "tinywasm/sqlite" .                    # vacío, todo el repo
grep -rn "tinywasm/context" .                    # vacío, todo el repo
grep -rn "\"strings\"\|\"strconv\"" tests/*.go   # vacío
grep -n "go:build" tests/*.go                    # vacío (ninguna partición !wasm/wasm sobrevive)
grep -rn "map\[" *.go                            # vacío fuera de fsm.go (§11 — cero map, sin excepciones nuevas)
test -f tests/tenant_isolation_test.go           # existe (§7.5b)
test -f tests/conformance_test.go                # existe (§7.5c)
gotest ./...                                     # verde
GOOS=js GOARCH=wasm gotest ./...                 # verde (o el equivalente TinyGo del repo)
```

---

## Etapa 8 — `go.mod` — estado final

### 8.1 Se quita

```
github.com/tinywasm/context  (indirecto también, verificar tras go mod tidy)
github.com/tinywasm/mcp
github.com/tinywasm/sqlite
github.com/tinywasm/unixid
replace github.com/tinywasm/time => ./internal/tinytime
```
`github.com/tinywasm/json` no aparece hoy como dependencia directa en el `go.mod` de este repo pero sí
se importaba en `mcp.go` (probablemente resuelta transitivamente) — confirma que no queda ninguna
referencia tras `go mod tidy`.

### 8.2 Se añade / actualiza a (ver §2 para el razonamiento de cada versión)

```
github.com/tinywasm/ddl    v0.0.4    (nueva, directa)
github.com/tinywasm/events v0.0.2    (nueva)
github.com/tinywasm/fmt    v0.25.3   (bump desde v0.23.2)
github.com/tinywasm/form   v0.2.16   (bump desde v0.2.0)
github.com/tinywasm/model  v0.0.16   (nueva, directa — antes solo transitiva vía orm)
github.com/tinywasm/orm    v0.11.1   (bump desde v0.7.1)
github.com/tinywasm/router v0.1.15   (nueva)
github.com/tinywasm/time   v0.5.0    (bump desde v0.4.0, sin replace)
github.com/tinywasm/view   v0.1.1    (nueva)
```

### 8.3 Pasos

```
go get github.com/tinywasm/ddl@v0.0.4 github.com/tinywasm/events@v0.0.2 \
  github.com/tinywasm/fmt@v0.25.3 github.com/tinywasm/form@v0.2.16 \
  github.com/tinywasm/model@v0.0.16 github.com/tinywasm/orm@v0.11.1 \
  github.com/tinywasm/router@v0.1.15 github.com/tinywasm/time@v0.5.0 \
  github.com/tinywasm/view@v0.1.1
go mod tidy
```

### 8.4 Criterio de aceptación final (repo completo)

```
grep -rn "tinywasm/mcp\|tinywasm/json\|tinywasm/unixid\|tinywasm/sqlite\|tinywasm/sqlt\|tinywasm/postgres\|tinywasm/layout\|tinywasm/context" .
# vacío en TODO el repo, tests incluidos

grep -n "replace" go.mod        # vacío
find . -iname "*tinytime*"      # vacío

grep -n "var _ router.OpModule = (\*Module)(nil)" ops.go   # presente
grep -n "func NewView" view.go                              # presente

go build ./...                       # limpio
GOOS=js GOARCH=wasm go build ./...   # limpio
gotest ./...                          # verde
```

---

## 9. Orden de ejecución

Etapa 0 no depende de ninguna otra y puede ejecutarse primero o en paralelo — es independiente.
Etapas 1→8 son estrictamente secuenciales (cada una asume que la anterior ya aplicó): 1 (modelo) antes
que 2/3 (que usan el casing puro resultante), 2/3 antes que 4 (Module.New ya con Deps.IDs/events.
Publisher), 4 antes que 5 (view usa los Ops/Args de la etapa 4), 6 puede ir junto con 3 (ambas tocan
`NewRepository`, aplícalas en el orden de este documento para no reescribir dos veces), 7 al final
(los tests ejercitan el resultado de 1-6), 8 al final de todo (`go mod tidy` limpia lo que ya dejó de
usarse).

## 10. Criterio de aceptación global

```bash
grep -rn "tinywasm/mcp\|tinywasm/json\|tinywasm/unixid\|tinywasm/sqlite\|tinywasm/sqlt\|tinywasm/postgres\|tinywasm/layout\|tinywasm/context" .
# vacío en TODO el repo, tests incluidos

grep -rn "internal/tinytime" .   # vacío
grep -n "replace" go.mod         # vacío

grep -n "EventPublisher" *.go              # vacío
grep -n "var _ router.OpModule" ops.go     # presente
grep -n "var _ SchedulingService" service.go  # presente

gotest ./...                          # verde
GOOS=js GOARCH=wasm gotest ./...      # verde
```

## 11. Fuera de alcance de este plan

- **`docs/PLAN_TESTS_BACKUP.md`** — backlog independiente de 20 casos de prueba (UC-01…UC-20) sobre
  lógica de negocio (validaciones, transiciones FSM, disponibilidad con excepciones, aislamiento
  multi-tenant). No es parte de esta migración de infraestructura — su alcance y vigencia son
  distintos (un backlog de cobertura vive más que una migración de una sola vez). Se actualizó por
  separado para reemplazar su mención a SQLite/`tinywasm/sqlite` por `storage/mem` (coherente con la
  Etapa 7 de este plan) — no fusiones su contenido aquí.
- **El `map[string]map[string]string` de `fsm.go`** — viola la regla "cero map" de `AGENTS.md`, pero
  es lógica de negocio ya correcta y testeada, no un puerto de infraestructura. Ver nota en
  `AGENTS.md` §"Domain-specific notes". No lo toques en este plan.
- **Los mensajes de error ricos que daba `mcp.Result{Content: msg}`** (p. ej. "Set the staff timezone
  first using upsert_calendar_config") se preservan de forma aproximada vía `writeError`/`ctx.Write`
  (Etapa 4 §4.4) porque `router.Context` no tiene un envelope de error-con-mensaje propio — si la app
  necesita una respuesta de error más rica (código + mensaje estructurado), es una mejora a
  `router.Context` aguas arriba, no un parche local en este módulo.
- **RBAC/autorización** — este módulo se mantiene fuera de alcance de IAM, sin cambios (ver
  `docs/ARCHITECTURE.md` §5).
