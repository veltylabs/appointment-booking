# PLAN — appointment_booking: migrar model.go a model.Definition

> This plan is dispatched via the CodeJob workflow. See skill: **agents-workflow**.

✅ **Desbloqueado.** `github.com/webtyp/model@v0.0.14` (con `orm@v0.9.28`) genera el helper
`<Struct>_.Campo` de forma **automática (always-on) para todo modelo con DB** — sin directiva. Ojo con
el **casing puro** (`tenant_id`→`TenantId`, `client_id`→`ClientId`, `..._id`→`...Id`; ya no `...ID`)
descrito en §5.

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`appointment_booking`). Plan
autocontenido: todo contrato, regla y ejemplo está inline.

---

## 1. Qué cambia y por qué

El ecosistema tinywasm invirtió la generación de modelos: se escribe una definición tipada
(`model.Definition`) a mano, y `ormc` genera el struct concreto + plomería. Migración **mecánica**:
mismo comportamiento, mismas columnas/tabla, mismo JSON. Este módulo tiene 5 structs con rol DB y 12
structs de transporte (args de handlers) sin rol DB.

## 2. Contrato de `github.com/webtyp/model` (inline)

`Field.Type` **no** es un literal de un enum — es la interfaz `Kind`. Se rellena llamando a un
constructor (`model.Text()`, `model.Int()`, …), nunca asignando `model.FieldText` directamente:

```go
package model

// FieldType es el mapeo determinista de almacenamiento/wire — lo devuelve Kind.Storage(),
// nunca se asigna directamente a Field.Type.
type FieldType int
const (
    FieldText FieldType = iota // string
    FieldInt                   // int64
    FieldFloat                 // float64
    FieldBool                  // bool
    FieldBlob                  // []byte
    FieldStruct                // struct anidado — Kind = model.Struct(ref)
    FieldIntSlice               // []int
    FieldStructSlice            // []T anidado — Kind = model.StructSlice(ref)
    FieldRaw                    // JSON pre-serializado
)

// Kind reemplaza el antiguo par Field.Type-enum + Field.Widget. Implementaciones
// sin estado, seguras para reuso concurrente.
type Kind interface {
    Storage() FieldType          // mapeo determinista a Go/DDL
    Name() string                // nombre semántico: "text", "int", "email", ...
    Validate(value string) error // SIEMPRE presente — fail-closed
}

// Constructores base — devuelven Kind, no un literal FieldType:
func Text() Kind  // storage FieldText
func Int() Kind   // storage FieldInt
func Float() Kind // storage FieldFloat
func Bool() Kind  // storage FieldBool
func Blob() Kind  // storage FieldBlob

type FieldDB struct { PK, Unique, AutoInc bool }

type Field struct {
    Name      string      // nombre EXPLÍCITO en wire/DB — tú lo escribes, ya no se deriva del struct
    Type      Kind        // model.Text(), model.Int(), ... — NUNCA un literal FieldType
    NotNull   bool
    OmitEmpty bool
    DB        *FieldDB    // nil = sin persistencia (structs de args/transporte)
    Ref       *Definition // solo FK escalar; en composición (Struct/StructSlice) el ref va
                          // en el constructor del Kind, no aquí
    Exclude   bool
    Permitted
}

type Fields = []Field

type Definition struct {
    Name   string
    Fields Fields
}
```

Mapeo fijo: `model.Text()`→`string`, `model.Int()`→`int64`, `model.Float()`→`float64`,
`model.Bool()`→`bool`. Variable de definición debe llamarse `<Struct>Model`.

**Ya no existe `Field.Widget`.** Un Kind con UI es un `Kind` de `github.com/webtyp/form/input`
(p. ej. `input.Text()`). Este módulo **sí** usa widgets hoy — en los 12 structs de args, no en
los 5 con rol DB (ver §4) — así que no basta con los Kinds base para todo el archivo.

**⚠️ Advertencia de nombres de columna (leer antes de tocar `Reservation`):** las columnas actuales
para `StaffIDSnapshot`/`ServiceIDSnapshot` son `staff_idsnapshot`/`service_idsnapshot` (SIN guión bajo
entre "id" y "snapshot" — una irregularidad del conversor snake_case actual, ya así en producción). El
nombre de columna ahora se **escribe explícito** en `Field.Name` — no se deriva. Para NO forzar un
rename de columnas en la base de datos, preserva exactamente `"staff_idsnapshot"` /
`"service_idsnapshot"` como `Field.Name` (ver §4). Como consecuencia, el identificador Go que `ormc`
derive de ese nombre puede NO coincidir exactamente con `StaffIDSnapshot`/`ServiceIDSnapshot` (la
conversión estándar snake→PascalCase de un nombre irregular como `idsnapshot` no reconstruye
`IDSnapshot`). **Verifica el nombre de campo generado tras correr `ormc`**: si difiere de
`StaffIDSnapshot`/`ServiceIDSnapshot`, ajusta los usos en `service.go`/`repository.go` al nombre nuevo
— NO renombres la columna para forzar que coincida.

---

## 3. Estado actual (`model.go`, a portar)

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

## 4. Estado objetivo (`model.go` reescrito)

Los 5 structs con rol DB de abajo (`EmployeeServiceConfig` … `Reservation`) **no tienen ningún
widget** en el `model_orm.go` actual — ni siquiera `price_override`/`price_snapshot` (`FieldFloat`).
No les inventes ninguno: preserva Kinds base (`model.X()`). Los 12 structs de args, en cambio,
**sí** tienen widget hoy, campo por campo — esos sí van con `input.X()` (ver después de
`ReservationModel`).

```go
package appointmentbooking

import (
	"github.com/webtyp/form/input"
	"github.com/webtyp/model"
)

var EmployeeServiceConfigModel = model.Definition{
	Name: "employee_service_config",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text()},
		{Name: "staff_id", Type: model.Text()},
		{Name: "service_id", Type: model.Text()},
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
		{Name: "tenant_id", Type: model.Text()},
		{Name: "staff_id", Type: model.Text()},
		{Name: "timezone", Type: model.Text()},
		{Name: "is_active", Type: model.Bool()},
	},
}

var WorkCalendarWeeklyModel = model.Definition{
	Name: "work_calendar_weekly",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text()},
		{Name: "staff_id", Type: model.Text()},
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
		{Name: "tenant_id", Type: model.Text()},
		{Name: "staff_id", Type: model.Text()},
		{Name: "specific_date", Type: model.Int()},
		{Name: "exception_type", Type: model.Text()},
		{Name: "start_time", Type: model.Int()},
		{Name: "end_time", Type: model.Int()},
		{Name: "notes", Type: model.Text()},
	},
}

// NOTA: "staff_idsnapshot" / "service_idsnapshot" preservan EXACTAMENTE el nombre de columna
// actual (irregularidad histórica, sin guión bajo) — no renombrar la columna. Ver advertencia §2.
var ReservationModel = model.Definition{
	Name: "reservation",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text()},
		{Name: "client_id", Type: model.Text()},
		{Name: "creator_user_id", Type: model.Text()},
		{Name: "employee_service_config_id", Type: model.Text()},
		{Name: "staff_idsnapshot", Type: model.Text()},
		{Name: "service_idsnapshot", Type: model.Text()},
		{Name: "duration_min_snapshot", Type: model.Int()},
		{Name: "price_snapshot", Type: model.Float()},
		{Name: "currency_snapshot", Type: model.Text()},
		{Name: "reservation_date", Type: model.Int()},
		{Name: "reservation_time", Type: model.Int()},
		{Name: "local_string_date", Type: model.Text()},
		{Name: "local_string_time", Type: model.Text()},
		{Name: "status", Type: model.Text()},
		{Name: "rescheduled_from_id", Type: model.Text()},
		{Name: "payment_id", Type: model.Text()},
		{Name: "notes", Type: model.Text()},
		{Name: "updated_at", Type: model.Int()},
		{Name: "updated_by", Type: model.Text()},
		{Name: "revision", Type: model.Int()},
	},
}

// Las 12 Definitions de abajo — TimeSlot … ListAvailabilityArgs — SÍ tienen widget hoy en el
// `model_orm.go` actual (campo `Widget:` de la API vieja), campo por campo. Preserva esa
// asignación exacta con `input.X()`; no la dejes caer o el form saldría vacío en silencio.

var TimeSlotModel = model.Definition{
	Name: "time_slot",
	Fields: model.Fields{
		{Name: "start_utc", Type: input.Number()},
		{Name: "end_utc", Type: input.Number()},
	},
}

var CreateReservationArgsModel = model.Definition{
	Name: "create_reservation_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: input.Text()},
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
		{Name: "tenant_id", Type: input.Text()},
		{Name: "id", Type: input.Text()},
	},
}

var ListReservationsByStaffArgsModel = model.Definition{
	Name: "list_reservations_by_staff_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: input.Text()},
		{Name: "staff_id", Type: input.Text()},
		{Name: "from", Type: input.Number()},
		{Name: "to", Type: input.Number()},
	},
}

var ListReservationsByClientArgsModel = model.Definition{
	Name: "list_reservations_by_client_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: input.Text()},
		{Name: "client_id", Type: input.Text()},
	},
}

var ChangeReservationStatusArgsModel = model.Definition{
	Name: "change_reservation_status_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: input.Text()},
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
		{Name: "tenant_id", Type: input.Text()},
		{Name: "before", Type: input.Number()},
	},
}

var UpsertCalendarConfigArgsModel = model.Definition{
	Name: "upsert_calendar_config_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: input.Text()},
		{Name: "staff_id", Type: input.Text()},
		{Name: "timezone", Type: input.Text()},
		{Name: "is_active", Type: input.Checkbox()},
	},
}

var UpsertWeeklyCalendarArgsModel = model.Definition{
	Name: "upsert_weekly_calendar_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: input.Text()},
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
		{Name: "tenant_id", Type: input.Text()},
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
		{Name: "tenant_id", Type: input.Text()},
		{Name: "exception_id", Type: input.Text()},
	},
}

var ListAvailabilityArgsModel = model.Definition{
	Name: "list_availability_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: input.Text()},
		{Name: "staff_id", Type: input.Text()},
		{Name: "config_id", Type: input.Text()},
		{Name: "from", Type: input.Number()},
		{Name: "to", Type: input.Number()},
	},
}
```

**Por qué estos 12 sí y los 5 de arriba no:** verificado contra el `model_orm.go` que este mismo
repo tiene generado *hoy*: `EmployeeServiceConfig`…`Reservation` no tienen ningún `Widget:`
asignado en ninguno de sus campos (tampoco `price_override`/`price_snapshot` — quedan como
`model.Float()`, sin widget, exactamente como hoy: **no** los conviertas a `input.Decimal()`, eso
sería inventar UI nueva que no existía). `TimeSlot`…`ListAvailabilityArgs` sí, campo por campo,
exactamente como quedó arriba. Dejarlos caer a Kinds base sin widget rompería en silencio
cualquier `form.New()` construido sobre ellos — el mismo defecto ya corregido en `service_catalog`.

## 5. Pasos

> **Dependencias:** `go get github.com/webtyp/model@v0.0.14 github.com/webtyp/orm@v0.9.28 github.com/webtyp/form@v0.2.15`
> (`model` directa nueva, antes solo se llegaba transitivamente vía `orm`; `form` ya era
> dependencia directa (v0.2.0) — se bumpea para regenerar los widgets de §4).

1. Reescribe `model.go` con el contenido de §4, **sin directivas** (`// orm:typed_fields` ya no existe).
2. Regenera `model_orm.go` con `ormc` (instalado/actual). Los 5 helpers de los structs con rol DB
   (`EmployeeServiceConfig_`, `WorkCalendarConfig_`, `WorkCalendarWeekly_`, `WorkCalendarException_`,
   `Reservation_`) se generan **automáticamente (always-on)**.
3. ⚠️ **Casing puro (sin diccionario de acrónimos)** — afecta helper y campos de struct:
   - `tenant_id`→`TenantId`, `client_id`→`ClientId`, `payment_id`→`PaymentId`,
     `employee_service_config_id`→`EmployeeServiceConfigId`, `config_id`→`ConfigId`,
     `exception_id`→`ExceptionId`, `id`→`Id` (ya **no** `...ID`).
   - **Columnas irregulares:** `staff_idsnapshot`→`StaffIdsnapshot`, `service_idsnapshot`→`ServiceIdsnapshot`
     (un solo token tras el `_`; **no** `StaffIDSnapshot`). Reemplaza la advertencia de §2 con este
     resultado concreto.
   - Actualiza `service.go`/`repository.go` a los nuevos nombres
     (`.Where(Reservation_.Status)` no cambia; los `.XxxID`→`.XxxId` sí). Columnas/JSON del wire NO cambian.
4. No hay cambios de tipo `int`→`int64` en este módulo (todos los campos numéricos ya eran `int64`).

## 6. Fuera de alcance

- No renombrar columnas (incluyendo la irregularidad `staff_idsnapshot`/`service_idsnapshot`).
- No cambiar comportamiento de negocio (`service.go`).
- **No añadir** la directiva `// orm:typed_fields` (ya no existe).
- No añadir widgets **nuevos** que no tuviera ya el `model_orm.go` actual (no le pongas widget a
  `EmployeeServiceConfig`…`Reservation`, ni `input.Decimal()` a `price_override`/`price_snapshot`:
  hoy ninguno lo tiene). Sí **preservar** los que ya existen en los 12 structs de args (§4).

## 7. Criterio de aceptación

- `gotest ./...` verde con `go.mod` en `model v0.0.14` / `orm v0.9.28` / `form v0.2.15`.
- `model_orm.go` regenerado compila, incluyendo los 5 helpers `<Struct>_` (automáticos) usados por
  `service.go`/`repository.go`, con casing puro (`Id`, `TenantId`, `StaffIdsnapshot`, …).
- Los 12 structs de args (`TimeSlot`…`ListAvailabilityArgs`) conservan sus widgets
  (`input.Text()`/`input.Number()`/`input.Checkbox()`, ver §4); los 5 structs con rol DB siguen
  sin ninguno.
- Los 17 `Definition` (5 DB + 12 args) presentes; `model.go` sin structs planos con tags `db:` ni directivas.

## 8. Etapas

| # | Etapa | Salida | Criterio |
|---|---|---|---|
| 1 | `go get` model v0.0.14 + orm v0.9.28 + form v0.2.15 + reescribir `model.go` | 17 Definitions de §4, sin directiva (12 structs de args conservan sus widgets `input.X()`; los 5 con rol DB sin widget) | compila |
| 2 | Regenerar `model_orm.go` | struct + plomería + 5 helpers `<Struct>_` (automáticos) | helpers presentes, casing puro |
| 3 | Actualizar casing en callers | `service.go`/`repository.go` (`.XxxID`→`.XxxId`, `StaffIdsnapshot`) | callers compilan sin rename de columna |
