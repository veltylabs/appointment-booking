# Índice de planes — appointment_booking

Este repo tiene dos planes pendientes, independientes entre sí. `codejob` despacha solo
`docs/PLAN.md` — cuando toque ejecutar uno, copia/mueve su contenido aquí (o referencia el
archivo correspondiente según lo soporte tu flujo de codejob) antes de despachar.

| Plan | Archivo | Estado | Bloqueante |
|---|---|---|---|
| Eliminar shim `internal/tinytime`, usar `github.com/tinywasm/time` directo | [PLAN_REMOVE_TINYTIME_SHIM.md](PLAN_REMOVE_TINYTIME_SHIM.md) | Listo para despachar — prerequisito (`tinywasm/time v0.5.0` con `Weekday`/`MidnightUTC`/`LocalMinutesToUnixUTC`) ya publicado | Ninguno — despachable ya |
| Migrar `model.go` a `model.Definition` (refactor de modelo tinywasm) | [PLAN_MODEL_MIGRATION.md](PLAN_MODEL_MIGRATION.md) | ✅ Desbloqueado — despachable | Resuelto: `tinywasm/orm@v0.9.24` (+ `fmt@v0.25.1`) lee `Definition` y genera `<Struct>_` **always-on**. Ojo **casing puro** (`ID`→`Id`, `..._id`→`...Id`) |

**Orden sugerido:** el shim de `internal/tinytime` no depende del refactor de modelo — puede
despacharse primero y de forma independiente. La migración de modelo **ya está desbloqueada**
(`tinywasm/orm@v0.9.24`).
