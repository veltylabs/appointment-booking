# PLAN: Migración completa al ecosistema tinywasm

## Objetivo

Eliminar toda dependencia de `stdlib "context"` y `stdlib "time"` del módulo
`appointment-booking`, que pertenece al ecosistema tinywasm y debe usar
exclusivamente sus paquetes propios.

---

## Diagnóstico del estado actual

### Problema 1 — `stdlib "context"` (soluble de inmediato)

`context_workaround.go` existe porque `service.go` y sus interfaces declaran todas
sus firmas con `context.Context` de la stdlib. `mcp.go` recibe `*tinyctx.Context`
del framework MCP y debe convertirlo con `ToStd(ctx)` para poder llamar al
servicio.

Archivos afectados:

| Archivo | Uso |
|---------|-----|
| `service.go` | `import "context"` · interfaz `SchedulingService` · interfaz `EventPublisher` · todas las implementaciones |
| `context_workaround.go` | puente `ToStd()` que implementa `stdlib.Context` |
| `mcp.go` | 11 llamadas a `ToStd(ctx)` |

El `ctx` recibido **nunca se usa internamente** en ningún método de la capa de
servicio: no se consulta deadline, no se verifica cancelación, no se lee ningún
valor. Solo se reenvía a `pub.Publish()`. El único motivo de existencia del
workaround es la incompatibilidad de tipo en las firmas.

**Solución**: cambiar todas las firmas a `*tinyctx.Context`, eliminar
`context_workaround.go`, quitar `ToStd()` de `mcp.go`.

---

### Problema 2 — `stdlib "time"` (requiere extensión de `tinywasm/time`)

`service.go` usa `stdlib "time"` para operaciones de calendario que
`tinywasm/time` todavía no expone:

| Línea | Operación stdlib | Propósito |
|-------|-----------------|-----------|
| 146 | `time.LoadLocation(tz)` | Cargar zona horaria IANA para convertir horas locales a UTC |
| 148 | `time.UTC` | Fallback cuando la zona no existe |
| 151 | `time.Unix(date, 0).UTC()` | Obtener componentes de fecha (año/mes/día) desde unix timestamp |
| 156 | `time.Date(y,m,d, h,min, 0,0, loc).Unix()` | Reconstruir unix timestamp con zona aplicada |
| 224 | `time.Unix(d, 0).UTC().Weekday()` | Obtener día de semana desde unix timestamp |
| 372–373 | `time.Unix(...).UTC()` + `time.Date(..., time.UTC).Unix()` | Calcular medianoche UTC de un día |

`tinywasm/time` actualmente provee: `Now() int64`, `FormatDate`, `FormatTime`,
`FormatDateTime`, `SetTimeZoneOffset`, `GetTimeZoneOffset`. Solo maneja **un
único offset global**, no zonas IANA por nombre.

**La función crítica** es `LocalIntToUnixUTC(date, localInt, tz)` que convierte
un horario laboral expresado en minutos-desde-medianoche en un timestamp UTC
considerando la zona horaria del staff. Sin equivalente en `tinywasm/time` esto
no puede migrar sin romper la lógica de negocio.

---

## Plan de ejecución en dos fases

### Fase 1 — Migrar `context` (sin dependencias externas)

**Alcance**: solo este módulo `appointment-booking`.

#### 1.1 — `service.go`

1. Reemplazar `import "context"` por `tinyctx "github.com/tinywasm/context"`.
2. Cambiar la interfaz `EventPublisher`:
   ```go
   // antes
   Publish(ctx context.Context, event string, payload any) error
   // después
   Publish(ctx *tinyctx.Context, event string, payload any) error
   ```
3. Cambiar la interfaz `SchedulingService`: todos los métodos de
   `context.Context` → `*tinyctx.Context`.
4. Cambiar todas las implementaciones de `schedulingService` (11 métodos) para
   recibir `*tinyctx.Context`.

#### 1.2 — `mcp.go`

Eliminar las 11 llamadas a `ToStd(ctx)`. Pasar `ctx` directamente:
```go
// antes
res, err := p.svc.CreateReservation(ToStd(ctx), cmd)
// después
res, err := p.svc.CreateReservation(ctx, cmd)
```

#### 1.3 — Eliminar `context_workaround.go`

El archivo deja de tener razón de existir.

#### Criterio de cierre

```
go build ./...   # sin errores
grep -r "ToStd\|stdlib.*context" . # sin resultados
```

---

### Fase 2 — Migrar `time` (requiere cambio en `tinywasm/time`)

**Prerequisito**: extender `tinywasm/time` con las siguientes funciones públicas
antes de modificar este módulo.

#### 2.1 — Funciones a agregar en `tinywasm/time`

```go
// Weekday retorna el día de semana (0=domingo … 6=sábado) de un unix timestamp UTC.
func Weekday(unixSec int64) int

// MidnightUTC retorna el timestamp unix de la medianoche UTC del día que
// contiene el timestamp dado.
func MidnightUTC(unixSec int64) int64

// LocalMinutesToUnixUTC convierte minutos-desde-medianoche en un timestamp UTC
// aplicando la zona horaria IANA indicada sobre la fecha dada (unix sec UTC).
// Si la zona no existe, cae a UTC.
func LocalMinutesToUnixUTC(dateSec int64, localMinutes int, tz string) int64
```

Estas funciones encapsulan `time.LoadLocation`, `time.Unix`, `time.Date` y
`time.Weekday` dentro de `tinywasm/time`, donde el uso de stdlib es legítimo
(es la implementación back-end de la capa de tiempo del ecosistema).

#### 2.2 — Cambios en `service.go` una vez disponibles

Reemplazar los bloques que usan stdlib time:

```go
// antes — LocalIntToUnixUTC usa time.LoadLocation / time.Date
func LocalIntToUnixUTC(date int64, localInt int, tz string) int64 {
    loc, err := time.LoadLocation(tz)
    ...
    localTime := time.Date(..., loc)
    return localTime.Unix()
}

// después
func LocalIntToUnixUTC(date int64, localInt int, tz string) int64 {
    return tinytime.LocalMinutesToUnixUTC(date, localInt, tz)
}
```

```go
// antes — ListAvailability usa time.Unix(d,0).UTC().Weekday()
tDate := time.Unix(d, 0).UTC()
dow := int(tDate.Weekday())

// después
dow := tinytime.Weekday(d)
```

```go
// antes — CreateReservation usa time.Unix + time.Date para medianoche
utcDate := time.Unix(cmd.SlotStartUTC, 0).UTC()
targetDay := time.Date(utcDate.Year(), ..., 0, 0, 0, 0, time.UTC).Unix()

// después
targetDay := tinytime.MidnightUTC(cmd.SlotStartUTC)
```

Tras estos cambios, `import "time"` desaparece completamente de `service.go`.

#### Criterio de cierre

```
go build ./...               # sin errores
grep -rn '"time"' service.go # sin resultados
grep -rn '"context"' .       # sin resultados (salvo go.sum)
```

---

## Resumen de archivos a modificar

| Archivo | Fase | Acción |
|---------|------|--------|
| `service.go` | 1 | Cambiar firmas de `context.Context` → `*tinyctx.Context`, cambiar import |
| `service.go` | 2 | Reemplazar usos de `stdlib/time` por funciones de `tinywasm/time` |
| `mcp.go` | 1 | Eliminar `ToStd()` en 11 call sites |
| `context_workaround.go` | 1 | **Eliminar archivo** |
| `tinywasm/time` (repo externo) | 2 | Agregar `Weekday`, `MidnightUTC`, `LocalMinutesToUnixUTC` |

## Orden de ejecución recomendado

```
Fase 1 (este módulo, autocontenida)
  └─ 1.1 service.go interfaces + implementaciones
  └─ 1.2 mcp.go — quitar ToStd()
  └─ 1.3 eliminar context_workaround.go

Fase 2 (depende de cambio en tinywasm/time)
  └─ 2.1 agregar funciones en tinywasm/time y publicar nueva versión
  └─ 2.2 actualizar go.mod con nueva versión de tinywasm/time
  └─ 2.3 service.go — reemplazar bloques time.*
  └─ 2.4 eliminar import "time" de service.go
```
