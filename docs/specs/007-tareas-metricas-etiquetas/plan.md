# Plan 007 — Tareas: nombre, métricas, etiquetas e historial

Plan de implementación de [spec.md](spec.md) (aprobada 2026-11-02). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias nuevas
Ninguna.

## Esquema (migración `00007_etiquetas.sql`, solo aditiva)
- **`labels`:** `id`, `name`, `name_normalized`, `color`, `position`, `retired_at`, fechas;
  nombre único entre las activas (RF-36, RF-37). Mismo patrón que `priorities`.
- **`work_item_labels`:** `(work_item_id, label_id)` como PK, `created_at`; `work_item_id` con
  `ON DELETE CASCADE`; índice por `label_id` para el filtro (RF-44).
- Las métricas y el historial no necesitan esquema: salen de `audit_log`, `work_items`,
  `statuses` y `work_item_dependencies`. Índice ya existente: `audit_log (target_type, target_id)`.

## Backend
- **`catalog`:** etiquetas con el mismo patrón que las prioridades (`mutate`, auditoría,
  `retire`/`restore`): rutas `/labels`; `Catalog.labels`. Mismo permiso de administración del catálogo.
- **`workitems`:**
  - `label_ids *[]uuid` en crear y modificar; si no llega, se conservan (clientes anteriores).
    Sin duplicados, máximo 10 (`too_many_labels`, RF-41); una etiqueta retirada o inexistente solo
    se acepta si ya la tenía (`label_retired`, RF-39).
  - Auditoría: `label_ids` en `auditView` antes y después, más `labels_added` y `labels_removed` (RF-46).
  - `Node` y `WorkItem` con `label_ids`; el árbol los carga en una consulta como los asignados.
  - Filtro del listado `label_id` (repetible): `EXISTS` sobre `work_item_labels` con `= ANY` (RF-44).
- **`projects/templates.go`:** copia las etiquetas activas (RF-45).
- **Métricas (`workitems/metrics.go`)**, `GET /work-items/{id}/metrics?today=YYYY-MM-DD`
  (`today` es la fecha local del navegador; sin ella, la fecha UTC del servidor):
  - **Plazo** (RF-9 a RF-15): días de `start_date` a `due_date` ambos incluidos; transcurridos =
    `today − start + 1`, acotado a `[0, total]`; esperado = `round(100·transcurridos/total)`;
    avance = `weighted_done_ratio` (100 si está cerrada); umbral ±10.
  - **Aporte** (RF-16 a RF-20): peso del subárbol = `agg_weight` (nodo incluido). Padre:
    `agg_weight` del padre. Proyecto: `sum(weight)` de sus work items. `share = 100·sub/total`;
    `points = share · avance derivado / 100`.
  - **Historial de estados:** se reconstruye de `audit_log` (`work_item.updated` con
    `before.status_id ≠ after.status_id`). El estado inicial es el `before.status_id` del primer
    registro `work_item.updated` (o el actual si no hay ninguno). Si hay cambios sin `status_id`
    en `before` (registros incompletos), lo que dependa de ello es «sin datos» (RF-35).
  - **Lead y cycle time** (RF-21 a RF-25): de `created_at` / primera salida del estado inicial al
    último paso a cerrado. En días con un decimal.
  - **Tiempo en el estado actual** (RF-23): desde el último cambio de estado (o `created_at`).
  - **Vencimiento** (RF-26 a RF-29): con `today` y `due_date`; si está cerrada, la fecha de
    `closed_at` en UTC.
  - **Reaperturas** (RF-30): pasos de cerrado a abierto en el historial de estados.
  - **Bloqueos** (RF-31 a RF-33): bloqueantes actuales con `OpenBlockers`; las referencias de
    los que el usuario no puede leer se devuelven sin clave ni título. Tiempo bloqueado: unión
    de intervalos en los que alguna dependencia `blocks` hacia la tarea existía (de
    `work_item.dependency_added`, o de `created_at` de la dependencia si no hay registro, hasta
    `work_item.dependency_removed`, el borrado del bloqueante o ahora) y el bloqueante estaba
    abierto (su propio historial de estados), recortada a los periodos en que la tarea estaba abierta.
- **Historial (`workitems/history.go`)**, `GET /work-items/{id}/history?before=<audit id>`:
  - Registros `work_item.created`, `work_item.updated`, `work_item.page_updated`,
    `work_item.dependency_added` y `work_item.dependency_removed` de la tarea, del más reciente al
    más antiguo; los comentarios no (ya están en su sección).
  - Una entrada por registro con la lista de propiedades cambiadas (`field`, `from`, `to`)
    comparando `before` y `after`. Los identificadores se resuelven a nombres (estado, tipo,
    prioridad, talla, etiqueta, persona, padre con su clave) en una consulta por clase (RF-49).
    `assignee_ids` sustituye a `assignee_id` cuando está.
  - La descripción es un cambio sin valores (RF-50). Entradas consecutivas de la misma persona
    cuyo único cambio es la descripción y que distan < 10 min se agrupan (RF-51).
  - Paginación por cursor sobre `audit_log.id`: se leen lotes de 100 hasta tener 50 entradas
    completas; `next` es el id del último registro consumido (RF-52).
  - Mismo permiso que leer la tarea (RF-53). El nombre del autor sale de `users`, que no se
    borra (RF-54).

## Frontend
- **Nombre** (RF-1 a RF-3): solo `locales/es.json`. Test de Vitest que recorre el catálogo y
  falla si algún valor contiene «work item». «Lista de tareas» pasa a «Lista de comprobación».
- **Tipos y API:** `Label`, `Catalog.labels`, `label_ids` en `WorkNode`, `WorkItem` y en la
  entrada de modificación (`toUpdateInput`); `CatalogKind` `labels`; `api/metrics.ts` y
  `api/history.ts` con TanStack Query.
- **`features/work/LabelsPicker.vue`:** como `AssigneesPicker` (búsqueda, casillas, guarda al
  cerrar), con chips de color; máximo 10. También sirve como filtro del árbol (modo sin guardar).
- **Ficha:** etiquetas entre las principales (RF-42); `MetricsSection.vue` entre propiedades y
  comentarios (RF-5), plegada, con el estado en `localStorage` (`useLocalStorage`, RF-7); la
  consulta de métricas se invalida al guardar la tarea (RF-8). `HistorySection.vue` al final,
  plegada, con «Cargar más» (RF-47, RF-52).
- **Árbol:** columna de etiquetas con `LabelsPicker` (RF-43) y filtro por etiquetas que pasa al
  modo lista (RF-44).
- **Catálogo:** tabla de etiquetas con `NamedCatalog` (nombre, color, posición).

## Tests
- **apitest:** catálogo de etiquetas (permisos, conflicto, retirar); etiquetas en tareas (máximo,
  retirada, filtro, auditoría, cliente anterior, plantilla); métricas (plazo y umbrales, sin
  plazo, aporte con peso 0 y sin peso, lead/cycle con reapertura, vencimiento, bloqueos con y sin
  acceso y tiempo bloqueado); historial (nombres, descripción agrupada, paginación, permisos);
  matriz de permisos; rendimiento (200 cambios < 300 ms).
- **Vitest:** sin «work item» en el catálogo; sección de métricas (plegado recordado, estados);
  etiquetas en ficha, árbol y filtro; historial y «Cargar más»; tabla de etiquetas.
