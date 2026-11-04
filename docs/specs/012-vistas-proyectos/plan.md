# Plan 012 — Vistas de proyectos

Plan de implementación de [spec.md](spec.md) (aprobada 2026-11-04). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias y esquema
No hay dependencias nuevas: el arrastre usa el arrastre nativo de HTML, como el kanban de tareas.

Migración `00011_vistas_proyectos.sql`. Es aditiva, así que la versión anterior de la API sigue
funcionando.
- **Tabla `project_statuses`**, con la misma forma que `statuses`:
  - columnas `name`, `name_normalized`, `color`, `position`, `is_closed`, `retired_at`, `created_at` y `updated_at`;
  - `is_initial`, que exige `NOT is_closed`, y `is_closing`, que exige `is_closed`;
  - ninguno de los dos puede estar retirado;
  - índices únicos parciales para el estado inicial, el de cierre y el nombre activo.
  - Se siembran los cinco estados neutros (RF-12).
- **Columna `projects.status_id`**, que referencia `project_statuses` y admite nulos.
  - Se rellena con «En ejecución» para los proyectos activos y archivados, y con «Cerrado» para los cerrados (RF-15, RF-16).
  - Es nullable para que la API anterior pueda seguir creando proyectos. Un nulo se lee como el estado inicial (`COALESCE`).
  - `CreateProject` la rellena con una subconsulta del estado inicial (RF-14).
- **Tabla `project_list_views`:**
  - columnas `id`, `owner_id` (nulo cuando es compartida), `name`, `name_normalized`, `config jsonb`, `created_by`, `created_at` y `updated_at`;
  - índices únicos de nombre por persona y para las compartidas, como `project_views`.

## Permisos
- **Nuevo `project_views:share`**, para el administrador y el líder: los roles que pueden crear proyectos (RF-71).
  - Va en la matriz de `authz` y en `apitest/matrix_work_test.go`.
- **Catálogo:** `catalog:manage`.
- **Cambiar el estado y las fechas:** `CanManageProject`.
- **Leer:** `work:read`.

## Backend
- **`catalog`:** estados de proyecto con el mismo patrón que los estados de tareas (`mutate`, auditoría `catalog.project_status.*`).
  - Rutas: `POST /project-statuses`, `PATCH /project-statuses/{id}` y `POST /project-statuses/{id}/retire|restore`.
  - `GET /catalog` gana `project_statuses`.
  - Para cambiar el estado inicial o el de cierre se quita primero la marca al que la tenía, como `ClearDefaultStatus`.
  - Quitarle la marca al estado que la tiene, o retirarlo, da `catalog_default_required` (RF-7). Con eso nunca falta un estado abierto ni uno cerrado (RF-6).
  - Cambiar `is_closed` de un estado en uso da `status_closed_locked`: su categoría decide el ciclo de vida.
- **`projects`:**
  - El DTO `Project` gana `status_id`.
  - **Nuevo `PUT /projects/{id}/status {version, status_id}`** (RF-17 a RF-25):
    - no vale para plantillas ni archivados;
    - el estado tiene que estar activo;
    - si la categoría cruza, cambia también `lifecycle`, con su auditoría `project.close` o `project.reopen`;
    - audita `project.status_changed` con el estado anterior y el nuevo;
    - sube `version`.
  - **`SetLifecycle`** (RF-21, RF-22): al cerrar, un estado abierto pasa al de cierre; al reabrir o desarchivar, un estado cerrado pasa al inicial.
- **Nuevo paquete `internal/projectboard`.**
  - **`GET /api/project-board?today=`:** todos los proyectos no archivados que no son plantilla, con lo que pintan los tres modos:
    - identificador, nombre, `status_id`, ciclo de vida, salud, equipo, responsable, fechas y versión;
    - `can_manage`;
    - el programa más cercano entre sus ancestros, o `null`;
    - las cifras: las propias o, si es un programa, las agregadas;
    - las fechas de su última línea base: el mínimo y el máximo de sus tareas;
    - sus hitos no cancelados, con `done`.
  - Las cifras salen de `portfolio.Load`. Los demás campos salen de dos consultas sqlc nuevas: `BoardProjects` (con la línea base) y `BoardMilestones`.
  - Los filtros, el orden, la agrupación y los recuentos por vista se calculan en el cliente sobre esta lista:
    - con 300 proyectos son pocos KB;
    - así «Yo», el semáforo y los recuentos por vista usan un único cálculo (RF-62 a RF-68, RF-72).
- **Vistas de proyectos** (`/api/project-views`, `GET|POST` y `PATCH|DELETE /{id}`), en el mismo paquete. Siguen a `views` de la spec 008:
  - `Config{mode, filters, sort, columns, group}`, con campos, modos y valores validados;
  - tope de 30 filtros con 50 valores cada uno;
  - límites de 50 vistas personales y 30 compartidas;
  - `view_name_conflict` y `too_many_views`;
  - solo se auditan las compartidas (RF-80).
- **Tests:**
  - apitest del catálogo (inicial y cierre obligatorios), del cambio de estado (cierra, reabre, conflicto, permisos, archivado), de `SetLifecycle` con el estado, del tablero, de las vistas y de la migración de los existentes;
  - filas en la matriz.

## Frontend
- **API:** `api/projectBoard.ts` (tablero, cambio de estado, vistas) y los estados de proyecto en `api/catalog`.
- **Catálogo:** sección «Estados de proyecto» en `CatalogPage`, igual que la de estados de tareas, con las marcas de inicial y de cierre.
- **`pages/ProjectsPage.vue` pasa a tener los tres modos:**
  - estado en la query: `mode`, `view` y los filtros;
  - recuerda en el navegador la última vista y el último modo (RF-79);
  - mantiene el botón «Desde plantilla».
- **Piezas comunes** en `features/projectviews/`:
  - `filters.ts`: campos, filtrado, orden y agrupación, como funciones puras con test (RF-62 a RF-68);
  - `ListViewsBar.vue`: las vistas con su recuento y «Cambios sin guardar».
  - `FilterBar` se generaliza con las props `fields`, `kinds` y el prefijo de las etiquetas.
- **`ProjectBoard.vue` (Kanban):**
  - columnas del catálogo más las retiradas en uso;
  - arrastre nativo solo cuando las columnas son por estado y el proyecto se puede gestionar;
  - un menú «Mover a…» como alternativa con teclado;
  - si algo falla, la tarjeta vuelve y se muestra el error;
  - tope de 100 por columna (RF-33 a RF-45).
- **`ProjectGantt.vue` (Cronograma):**
  - eje con la escala elegida, grupos con su agregado (`portfolio.Combine` no está en el cliente, así que el agregado se pide en el tablero, por programa), línea base, hitos y línea de hoy;
  - arrastre de los extremos o de la barra entera con eventos de puntero, guardado con el `PATCH` completo del proyecto (RF-46 a RF-59);
  - flechas del teclado sobre la barra enfocada como alternativa.
- **`ProjectTable.vue`:** la tabla actual movida aquí, alimentada por el tablero filtrado y con las columnas nuevas (RF-60, RF-61).
- **Ficha:** un selector de estado en `ProjectSummary` (RF-26).
- **Textos:** `projectViews.*` y `catalog.projectStatuses.*` en `es.json`.
- **Vitest:** funciones de filtro, Kanban (columnas, arrastre, «Mover a…», error), Cronograma (barras, agrupación, arrastre), vistas y página (modo en la URL).

## Riesgos
- **Los proyectos que crea la API anterior quedan con estado nulo.** Se tratan como el estado inicial al leer.
- **El tablero calcula el plan de todos los proyectos en cada consulta.** `portfolio.Load` ya lo hace para el portafolio; la medición queda en la demo.
