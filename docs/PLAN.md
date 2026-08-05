---
PLAN: "fix: appointment_booking retarget to current view API, remove 3 map violations, translate comments"
TAG: v0.1.1
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 12383940706352371258
PR: https://github.com/veltylabs/appointment_booking/pull/9
---

> This plan is dispatched via the CodeJob workflow. See skill: **agents-workflow**.

# PLAN — appointment_booking: retargeting + 3 map violations + traducción de comentarios

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`appointment_booking`).
El código actual en este repo (`view.go`, `ops.go`, `model.go`, `fsm.go`, `repository.go`,
`service.go`, `ssr.go`) es una migración completa y funcional al arnés reutilizable, ya escrita — este
plan **no la rehace**, solo corrige 4 problemas puntuales descubiertos en revisión, todos ya
verificados end-to-end (`go build`, `GOOS=js GOARCH=wasm go build`, `gotest ./...`) en un entorno de
prueba antes de escribir este plan.

## 1. Por qué existe este plan

1. **`view.go` usa una API de `tinywasm/view` ya eliminada** (`v0.1.1`: `WithFill` + proyector
   `func(list model.FielderSlice) []view.Item`). El `go.mod` actual pinea esa versión vieja a
   propósito, así que compila hoy — pero se rompe en cuanto alguien actualice las dependencias,
   exactamente como pasó en `item_catalog`/`business_hours`/`clinical_encounter` antes de corregirlos
   esta misma sesión.
2. **Tres usos de `map[K]V`** (`fsm.go`'s `transitions`, y dos en `service.go`'s `ListAvailability`)
   violan `AGENTS.md`: *"No Go `map[K]V` anywhere, test code included"* — sin excepciones.
3. **`ssr.go`'s `IconSvg()` devuelve `map[string]string`** — un cuarto uso de map, y además usa un
   patrón de registro de ícono obsoleto. El patrón correcto vive en `github.com/tinywasm/svg` (ver
   su README): `svg.Icon` (referencia, sin build tag) + `svg/sprite.Define`/`Path` (geometría, en
   `svg.go` con `//go:build !wasm`) — nunca un `map[string]string` a mano.
4. **Comentarios en inglés** en `service.go`, `repository.go`, `model.go`, `ops.go`, `view.go`,
   `fsm.go` — el resto del batch ya tradujo los suyos a español, manteniendo el código (identificadores)
   en inglés.

Todo lo de abajo fue verificado compilando y corriendo `gotest ./...` contra las dependencias
publicadas reales — no es especulativo.

## 2. Bump de dependencias

```bash
go get github.com/tinywasm/ddl@v0.0.7 github.com/tinywasm/events@v0.0.2 github.com/tinywasm/fmt@v0.25.5 \
  github.com/tinywasm/form@v0.3.13 github.com/tinywasm/model@v0.1.2 github.com/tinywasm/orm@v0.11.4 \
  github.com/tinywasm/router@v0.1.19 github.com/tinywasm/time@v0.5.2 github.com/tinywasm/view@v0.1.12 \
  github.com/tinywasm/svg@latest
go mod tidy
```

(Re-verifica contra `item_catalog@main`'s `go.mod` por si estas versiones avanzaron desde que se
escribió este plan.) `github.com/tinywasm/svg` es nuevo (§4). `go mod tidy` reintroduce
`github.com/tinywasm/dom`/`github.com/tinywasm/json` como indirectos — es correcto, no los toques.

## 3. `view.go` — reemplazo completo (API de `view.New` actual)

`view.New` ya no acepta el proyector de 4to argumento ni `WithFill` — el registro implementa
`view.Itemizer` (`Item() view.Item`) directamente, y la selección se resuelve con un índice interno
que el `Presenter` construye solo en `Reload()`. Reemplaza `view.go` completo:

```go
package appointmentbooking

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// Item implementa view.Itemizer — el ÚNICO código específico de view que carga este registro. El
// Presenter indexa las filas por ID a partir de esto durante Reload; no hay lookup manual byId/WithFill.
func (r *Reservation) Item() view.Item {
	return view.Item{
		ID:          r.Id,
		Label:       r.LocalStringDate + " " + r.LocalStringTime,
		Description: r.Status,
	}
}

// NewView construye el Presenter de Reservation — acotado al horario de un solo staff, ya que no
// existe una operación "listar todas las reservas de un tenant" sin acotar (ver
// docs/ARCHITECTURE.md §7). Solo lista: sin capacidad Saver/Deleter (las reservas solo mutan vía las
// transiciones FSM-guardadas de ChangeReservationStatus, y nunca se eliminan físicamente).
func NewView(caller router.Caller, tenantId, staffId string) view.Presenter {
	record := &Reservation{}

	return view.New(
		caller,
		record,
		OpListReservationsByStaff,
		func() model.ModelSlice { return &ReservationList{} },
		view.WithTitle("Reservas"),
		view.WithArgs(func() model.Encodable {
			return &ListReservationsByStaffArgs{TenantId: tenantId, StaffId: staffId}
		}),
	)
}
```

## 4. `tests/conformance_test.go` — `CanSave`/`CanDelete` ya no existen

`view.Presenter` no tiene esos métodos — `Saver`/`Deleter` son capacidades que el renderer descubre
por type assertion (comentario de doc de `view.Presenter`). Agrega `"github.com/tinywasm/view"` al
bloque de imports y reemplaza el bloque 4 de `TestViewConformance`:

```go
// ANTES
	// 4. Save and delete capabilities must be disabled by design
	if pres.CanSave() {
		t.Fatalf("expected CanSave to be false")
	}

	if pres.CanDelete() {
		t.Fatalf("expected CanDelete to be false")
	}
}

// DESPUÉS
	// 4. Save and delete capabilities must be disabled by design — Saver/Deleter are capabilities
	// the renderer discovers by type assertion (view.Presenter doc comment), not CanSave()/CanDelete().
	if _, ok := pres.(view.Saver); ok {
		t.Fatalf("expected no Saver capability")
	}

	if _, ok := pres.(view.Deleter); ok {
		t.Fatalf("expected no Deleter capability")
	}
}
```

## 5. `ssr.go` → `svg.go`: quitar el `map[string]string`, usar el patrón real de `tinywasm/svg`

**Borra `ssr.go` por completo** (solo tenía `IconSvg()`, nada más sobrevive). En su lugar:

**Nuevo archivo `svg.go`:**
```go
//go:build !wasm

package appointmentbooking

import "github.com/tinywasm/svg/sprite"

// IconSvg registra el ícono de marca del módulo. tinywasm/ssr lo extrae durante SSR y assetmin lo
// inyecta inline en <body> — nunca se llama a mano.
func (m *Module) IconSvg() *sprite.Sprite {
	return sprite.NewSprite(
		sprite.Define(iconAppointmentBooking, "0 0 16 16",
			sprite.Path("m10.29 11.71-3.293-3.293v-4.414h2v3.586l2.707 2.707zm-2.293-11.71c-4.418 0-8 3.582-8 8s3.582 8 8 8 8-3.582 8-8-3.582-8-8-8zm0 14c-3.314 0-6-2.686-6-6s2.686-6 6-6 6 2.686 6 6-2.686 6-6 6z"),
		),
	)
}
```

**En `service.go`**, agrega `"github.com/tinywasm/svg"` al bloque de imports y esta constante justo
después del bloque `var (ErrCalendarConfigNotFound...)` (la referencia al ícono vive en código
compartido sin build tag — solo el *nombre* llega al binario wasm, la geometría se queda en
`svg.go`):

```go
// iconAppointmentBooking es la referencia al ícono de marca del módulo — solo el nombre llega al
// binario wasm; la geometría se declara en svg.go (registrado por IconSvg en svg.go).
const iconAppointmentBooking = svg.Icon("appointment-booking-module")
```

Después de este cambio, verifica que `tinywasm/svg/sprite` no se filtra al build de wasm (debe
imprimir vacío):
```bash
GOOS=js GOARCH=wasm go list -deps ./... | grep tinywasm/svg/sprite
```

## 6. `fsm.go` — reemplazo completo (quita el `map[string]map[string]string`)

```go
package appointmentbooking

import "github.com/tinywasm/fmt"

// Estados
const (
	StatusPending     = "PENDING"
	StatusConfirmed   = "CONFIRMED"
	StatusCancelled   = "CANCELLED"
	StatusCompleted   = "COMPLETED"
	StatusNoShow      = "NO_SHOW"
	StatusExpired     = "EXPIRED"     // reserva no pagada que expiró (disparador: scheduler externo vía MCP)
	StatusRescheduled = "RESCHEDULED" // reserva original reemplazada por una nueva (registro de auditoría)
)

// Eventos
const (
	EventConfirm    = "CONFIRM"
	EventCancel     = "CANCEL"
	EventComplete   = "COMPLETE"
	EventNoShow     = "NO_SHOW_EVENT"
	EventExpire     = "EXPIRE"
	EventReschedule = "RESCHEDULE" // marca la original como RESCHEDULED; la nueva reserva se crea atómicamente
)

// transition es una fila de la tabla de transiciones FSM: (estado actual, evento) -> estado siguiente.
// Slice, no map — la regla "cero map" de AGENTS.md no tiene excepciones; esta tabla tiene 9 filas,
// un scan lineal no cuesta nada medible.
type transition struct {
	From  string
	Event string
	To    string
}

var transitions = []transition{
	{StatusPending, EventConfirm, StatusConfirmed},
	{StatusPending, EventCancel, StatusCancelled},
	{StatusPending, EventExpire, StatusExpired},
	{StatusPending, EventReschedule, StatusRescheduled},
	{StatusConfirmed, EventCancel, StatusCancelled},
	{StatusConfirmed, EventComplete, StatusCompleted},
	{StatusConfirmed, EventNoShow, StatusNoShow},
	{StatusConfirmed, EventReschedule, StatusRescheduled},
	// CANCELLED, COMPLETED, NO_SHOW, EXPIRED, RESCHEDULED son terminales — sin transiciones salientes
}

// ErrInvalidTransition se devuelve cuando una transición no está permitida.
var ErrInvalidTransition = fmt.Err("invalid", "transition")

// Transition devuelve el siguiente estado, o un error si la transición no es válida.
func Transition(current, event string) (string, error) {
	if IsTerminal(current) {
		return "", ErrInvalidTransition
	}
	for _, t := range transitions {
		if t.From == current && t.Event == event {
			return t.To, nil
		}
	}
	return "", ErrInvalidTransition
}

// IsTerminal devuelve true si el estado no tiene transiciones salientes.
func IsTerminal(status string) bool {
	switch status {
	case StatusCancelled, StatusCompleted, StatusNoShow, StatusExpired, StatusRescheduled:
		return true
	default:
		return false
	}
}
```

**Verificado:** `gotest ./...` sigue verde con este reemplazo — ninguna prueba dependía de la
representación interna de `transitions`, solo del comportamiento de `Transition`/`IsTerminal`.

## 7. `service.go` — los otros 2 `map[K]V`, dentro de `ListAvailability`

Ambos están en el mismo método. Verificado contra `tests/availability_runner_test.go` (203 líneas,
la suite dedicada a `ListAvailability`) — sigue en verde después de este cambio, el comportamiento es
idéntico, solo cambia la estructura de datos interna.

**7.1 — `activeWeeklies`: de `map[int]WorkCalendarWeekly` a slice + helper de scan lineal**

```go
// ANTES
	activeWeeklies := make(map[int]WorkCalendarWeekly)
	for _, w := range weeklies {
		if w.IsActive {
			activeWeeklies[int(w.DayOfWeek)] = w
		}
	}

// DESPUÉS
	// Slice, no map — la regla "cero map" de AGENTS.md no tiene excepciones; a lo sumo 7 filas
	// (una por día de la semana), un scan lineal no cuesta nada medible.
	activeWeeklies := make([]WorkCalendarWeekly, 0, len(weeklies))
	for _, w := range weeklies {
		if w.IsActive {
			activeWeeklies = append(activeWeeklies, w)
		}
	}
```

Agrega este helper nuevo, justo antes de `func (m *Module) ListAvailability(...)`:

```go
// weeklyForDay busca el horario semanal activo para un día de la semana dado — scan lineal sobre
// como mucho 7 filas, ver la nota en ListAvailability.
func weeklyForDay(weeklies []WorkCalendarWeekly, dow int) (WorkCalendarWeekly, bool) {
	for _, w := range weeklies {
		if int(w.DayOfWeek) == dow {
			return w, true
		}
	}
	return WorkCalendarWeekly{}, false
}
```

Y el único punto de consumo, más abajo en el mismo método:

```go
// ANTES
		weekly, hasWeekly := activeWeeklies[dow]
		if !hasWeekly {
			continue // skip day
		}

// DESPUÉS
		weekly, hasWeekly := weeklyForDay(activeWeeklies, dow)
		if !hasWeekly {
			continue // saltar el día
		}
```

**7.2 — `exceptionsByDate`: de `map[int64][]WorkCalendarException` a filtro inline sobre `exceptions`**

```go
// ANTES
	exceptionsByDate := make(map[int64][]WorkCalendarException)
	for _, e := range exceptions {
		exceptionsByDate[e.SpecificDate] = append(exceptionsByDate[e.SpecificDate], e)
	}

// DESPUÉS — el bloque completo se borra; el agrupamiento se hace inline en el punto de consumo (ver abajo)
	// Sin agrupar en un map — el filtro por fecha se hace inline con un scan lineal sobre
	// `exceptions` dentro del loop de días (ver más abajo). Las excepciones son un conjunto
	// disperso por diseño (feriados, horarios especiales); nunca lo suficientemente grande
	// para que un scan lineal importe.
```

Y el punto de consumo:

```go
// ANTES
		dayExceptions := exceptionsByDate[d]

// DESPUÉS
		var dayExceptions []WorkCalendarException
		for _, e := range exceptions {
			if e.SpecificDate == d {
				dayExceptions = append(dayExceptions, e)
			}
		}
```

## 8. Traducir comentarios a español — el resto de `service.go`, `repository.go`, `model.go`, `ops.go`

Mismo criterio del resto del batch: solo comentarios, nunca identificadores ni strings de negocio.

**`ops.go`** — un bloque:
```go
// ANTES
// writeError maps known sentinel errors to an HTTP-ish status code and writes err.Error() as the
// body, preserving (loosely) the human-readable messages the old mcp.Result{Content: msg} gave —
// router.Context has no error-with-message envelope of its own, so this is the module's own,
// minimal convention. See docs/PLAN.md §4 "Fuera de alcance" for why this isn't richer.

// DESPUÉS
// writeError mapea los errores sentinela conocidos a un código de estado tipo HTTP y escribe
// err.Error() como cuerpo, preservando (de forma laxa) los mensajes legibles que daba el viejo
// mcp.Result{Content: msg} — router.Context no tiene un envoltorio propio de error-con-mensaje,
// así que esta es la convención mínima propia del módulo. Deliberadamente no es más rica que esto.
```
(La línea siguiente, "Convención de mapeo...", ya está en español — no la toques.)

**`model.go`** — un bloque (nota: quita la referencia "Ver §1.1" al plan original ya borrado):
```go
// ANTES
// NOTA: "staff_idsnapshot" / "service_idsnapshot" preservan EXACTAMENTE el nombre de columna
// actual (irregularidad histórica, sin guión bajo) — NO renombrar la columna. Ver §1.1.

// DESPUÉS
// NOTA: "staff_idsnapshot" / "service_idsnapshot" preservan EXACTAMENTE el nombre de columna
// actual (irregularidad histórica, sin guión bajo) — NO renombrar la columna, la tabla ya existe
// con ese nombre en producción.
```

**`repository.go`** — cuatro bloques (los divisores `// ----` + nombre de entidad NO se traducen,
son nombres de tipo):
```go
// ANTES
// Package-level sentinel errors
...
// Repository provides CRUD operations for all appointment-booking tables.
...
// NewRepository creates a new Repository and migrates its 5 owned tables when the backend
// supports DDL (a no-op against storage/mem, used by this module's own tests).
...
	// Try to find existing record for this (tenantId, staffId)
	...
		// Does not exist — create
	...
	// Exists — update in place (preserve original ID)

// DESPUÉS
// Errores sentinela a nivel de paquete
...
// Repository provee operaciones CRUD para todas las tablas de appointment-booking.
...
// NewRepository crea un nuevo Repository y migra sus 5 tablas propias cuando el backend
// soporta DDL (no-op contra storage/mem, usado por las pruebas propias de este módulo).
...
	// Intenta encontrar un registro existente para este (tenantId, staffId)
	...
		// No existe — crear
	...
	// Existe — actualizar en el lugar (preservando el ID original)
```

**`service.go`** — el resto de comentarios en inglés (fuera de los ya cubiertos en §5/§7), todos
frases cortas y directas, mismo criterio:

| Antes | Después |
|---|---|
| `// Domain events emitted by this module.` | `// Eventos de dominio emitidos por este módulo.` |
| `// StaffReader verifies a staff member exists and belongs to the tenant.` | `// StaffReader verifica que un miembro del staff existe y pertenece al tenant.` |
| `// CatalogReader verifies a service exists and belongs to the tenant.` | `// CatalogReader verifica que un servicio existe y pertenece al tenant.` |
| `// DirectoryReader verifies a client exists and belongs to the tenant.` | `// DirectoryReader verifica que un cliente existe y pertenece al tenant.` |
| `// Calendar management` | `// Gestión de calendario` |
| `// Availability` | `// Disponibilidad` |
| `// Reservations` | `// Reservas` |
| `// Must check if CalendarConfig exists first` | `// Primero hay que verificar si CalendarConfig existe` |
| `// 1. Load WorkCalendarConfig` | `// 1. Cargar WorkCalendarConfig` |
| `// 2. Load WorkCalendarWeekly` | `// 2. Cargar WorkCalendarWeekly` |
| `// 3. Load WorkCalendarException` | `// 3. Cargar WorkCalendarException` |
| `// 4. Load existing Reservations` | `// 4. Cargar Reservations existentes` |
| `// 5. Load EmployeeServiceConfig` (dentro de `ListAvailability`) | `// 5. Cargar EmployeeServiceConfig` |
| `// 6. For each day D in [from, to] (assuming from and to are midnight UTC timestamps)` | `// 6. Para cada día D en [from, to] (asumiendo que from y to son timestamps de medianoche UTC)` |
| `// We increment by 1 day (86400 seconds)` | `// Incrementamos de a 1 día (86400 segundos)` |
| `// Apply exceptions` | `// Aplicar excepciones` |
| `// Priority: HOLIDAY > SPECIAL_HOURS > BLOCKED` | `// Prioridad: HOLIDAY > SPECIAL_HOURS > BLOCKED` |
| `// Generate slots` | `// Generar slots` |
| `// Check break` | `// Verificar break` |
| `// skip to the end of the break to allow slots after break` | `// saltar al final del break para permitir slots después del break` |
| `// Check blocked exceptions` | `// Verificar excepciones bloqueadas` |
| `// Overlap check` (aparece 2 veces, en el bloque de bloqueos y en el de reservas) | `// Verificación de solapamiento` |
| `// Check existing reservations` | `// Verificar reservas existentes` |
| `// 1. Load EmployeeServiceConfig` (dentro de `CreateReservation` — encabezado distinto al de arriba, mismo texto) | `// 1. Cargar EmployeeServiceConfig` |
| `// 2. Validate client` | `// 2. Validar cliente` |
| `// 3. Validate staff` | `// 3. Validar staff` |
| `// 4. Validate service` | `// 4. Validar servicio` |
| `// 5. Check availability` | `// 5. Verificar disponibilidad` |
| `// Get availability for the target day (midnight UTC)` | `// Obtener disponibilidad para el día objetivo (medianoche UTC)` |
| `// Broaden the search by one day on each side to account for timezone boundary differences` | `// Ampliar la búsqueda un día a cada lado para cubrir diferencias de límite de zona horaria` |
| `// Do an in-tx insert instead of repo.InsertReservation which uses db.Create` | `// Hacer un insert dentro de la tx en vez de repo.InsertReservation, que usa db.Create` |
| `// fetch updated` | `// obtener actualizado` |

## 9. Fuera de alcance

- No tocar `docs/ARCHITECTURE.md`/`README.md` salvo que ya describan `WithFill`/`CanSave`/`CanDelete`
  o el `map[string]string` de `IconSvg` — en ese caso, corrígelos para que coincidan con esta versión.
- No perseguir cobertura de tests más allá de lo que ya existe — este plan es correctividad de API +
  eliminación de maps + idioma, no una ronda de cobertura (la cobertura actual, ~41%, queda fuera de
  alcance aquí; si se decide perseguirla es un plan separado, siguiendo el mismo criterio de "solo
  pruebas de valor real" que el resto del batch).
- No renombrar `ops.go`/`service.go`/`repository.go`/`fsm.go` — solo `ssr.go` se reemplaza por `svg.go`
  (§5), el resto de nombres de archivo no cambian.

## 10. Criterio de aceptación

- `grep -rn "map\[" --include=*.go .` → vacío (ni `fsm.go`, ni `service.go`, ni `ssr.go`/`svg.go`).
- `test -f ssr.go` → no existe. `test -f svg.go` → existe.
- `grep -n "WithFill\|CanSave\|CanDelete" -r --include=*.go .` → vacío.
- `GOOS=js GOARCH=wasm go list -deps ./... | grep tinywasm/svg/sprite` → vacío.
- `go build ./...` y `GOOS=js GOARCH=wasm go build ./...` limpios.
- `gotest ./...` verde (incluye `tests/availability_runner_test.go`, que ejercita directamente la
  lógica que cambió en §7).
- `grep -rn "^\s*//" service.go repository.go model.go ops.go view.go fsm.go` — ningún comentario en
  inglés remanente.
- `git status` limpio tras el commit.
