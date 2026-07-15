# PLAN: Eliminar internal/tinytime — usar github.com/tinywasm/time directamente

## Problema

El agente anterior cometió un error: en lugar de agregar las funciones faltantes
al paquete real `github.com/tinywasm/time`, creó un subpaquete local
`internal/tinytime` que es una copia completa del paquete con las funciones
extra añadidas, y redirigió el import mediante un `replace` en `go.mod`:

```
replace github.com/tinywasm/time => ./internal/tinytime
```

Esto viola el principio del ecosistema tinywasm: los módulos propios no forcan
sus dependencias ni duplican paquetes del ecosistema.

---

## Prerequisito

Las funciones `Weekday`, `MidnightUTC` y `LocalMinutesToUnixUTC` deben existir
en `github.com/tinywasm/time` antes de ejecutar este plan.

Ver: `/home/cesar/Dev/Project/tinywasm/time/docs/PLAN.md`

Este plan asume que `github.com/tinywasm/time v0.5.0` ya fue publicado.

---

## Plan de ejecución

### Paso 1 — Eliminar `internal/tinytime`

```bash
rm -rf internal/tinytime
```

### Paso 2 — Limpiar `go.mod`

Quitar la línea:
```
replace github.com/tinywasm/time => ./internal/tinytime
```

Actualizar la versión requerida:
```
github.com/tinywasm/time v0.5.0
```

### Paso 3 — Actualizar dependencias

```bash
go get github.com/tinywasm/time@v0.5.0
go mod tidy
```

### Paso 4 — Verificar

`service.go` no requiere cambios. Ya usa los símbolos correctos:
- `tinytime.LocalMinutesToUnixUTC` — línea 145
- `tinytime.Weekday` — línea 212
- `tinytime.MidnightUTC` — línea 359

```bash
go build ./...
grep -r "internal/tinytime" .   # sin resultados
grep "replace" go.mod           # sin resultados
```

---

## Archivos a modificar

| Archivo | Acción |
|---------|--------|
| `go.mod` | Quitar `replace`, actualizar a `v0.5.0` |
| `internal/tinytime/` | Eliminar directorio completo |
