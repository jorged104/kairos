# Plan — Spec 024: comparar líneas base

## Datos
- Migración `00021_comparar_lineas_base.sql` (aditiva): `project_baseline_items` gana `item_key` y `title`, y pierde la clave ajena a `work_items` para conservar las tareas borradas (RF-24, RF-25). La versión anterior de la API lee las fechas cruzando con `work_items`, así que una tarea borrada sigue sin contar.
- `CopyBaselineItems` guarda la clave y el título. `BoardBaselineDates` cruza con `work_items` para no contar las borradas (RF-26).
- Consultas nuevas en `schedule.sql`: `GetBaselineByVersion`, `BaselineCompareItems` (cada tarea de la versión tal como está ahora; las borradas solo si guardaron título) y `CurrentCompareItems` (Hoy).

## API
- `GET /api/projects/{id}/baseline/compare?from=&to=`: cada lado es un número de versión o `today`. Sin parámetros, RF-3 y RF-4. Mismo lado: `validation_failed`; versión inexistente o proyecto sin línea base: `not_found`.
- Respuesta: `before`, `after` (null = Hoy), `versions`, `summary` (añadidas, quitadas, movidas, sin cambios, fin de cada lado y diferencia) e `items` ya ordenados (RF-15).
- Visibilidad con `projects.Get` y, para tareas en otro proyecto, `Principal.SeesProject` (spec 023). `id` es null cuando la fila no se puede abrir.
- Permiso: el de ver el proyecto (`work:read` + visibilidad). Fila nueva en la matriz.

## Frontend
- Ruta `/projects/:id/baseline-compare?from=&to=` con `BaselineCompare.vue` en `features/schedule`.
- Acción «Comparar» en `BaselinePanel.vue` cuando hay línea base.
- Selectores de lado que escriben en la dirección (RF-8), resumen, filtros por tipo de cambio (por defecto sin «Sin cambios») y tabla.
- Textos en `es.json` bajo `schedule.compare`.
