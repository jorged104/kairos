# Plan 004 — Proyectos, metodologías, hitos, work items y objetivos

Plan de implementación de [spec.md](spec.md) (aprobada 2026-10-23).
Aquí van las decisiones técnicas; el comportamiento lo fija la spec. Las «Decisiones de diseño
acordadas» y las «Decisiones de la revisión» de la spec son restricciones de entrada.

## Punto de partida

- **Spec 003** (rama `feat/003-usuarios`, PR #7 sin fusionar): sesión, CSRF, `authz` con la
  matriz única y el `Principal` (roles, permisos, `ScopeTeams` = equipos liderados + subequipos),
  `audit.Record` en la transacción, `apperr` con `details`, `db.Store.InTx`, `dbtest`,
  `apitest` (app completa contra BD temporal) y atributos dinámicos con validación por tipo
  (`attributes.Normalize`). Esta rama sale de ella; se fusiona después de la 003.
- No hay tablas de trabajo, editor de texto enriquecido ni eventos de dominio.
- La previsualización aplica migraciones en cada despliegue; esta spec **no** añade variables
  de entorno.

## Dependencias nuevas

| Paquete | Uso |
|---|---|
| `@tiptap/vue-3`, `@tiptap/pm`, `@tiptap/starter-kit`, `@tiptap/extension-table` (v3, versión exacta al instalar) | Editor y visor de descripciones (RF-70). Ya está en el stack de `AGENTS.md`. StarterKit v3 incluye `Link`. |
| shadcn-vue: `dialog`, `tabs`, `textarea`, `checkbox`, `dropdown-menu`, `progress`, `tooltip` | Pantallas nuevas (se generan en `node:24-alpine`, como en 003). |

Backend sin dependencias nuevas: pgx, sqlc, Squirrel y validator ya están.

## Decisiones técnicas

### Jerarquías (D3, D5)
Consulta recursiva (`WITH RECURSIVE`) sobre `parent_id`, sin tabla de cierre. Con profundidad
≤ 10 (work items) y ≤ 5 (proyectos) y los índices de `parent_id`, el subárbol de 5 000 work
items es una sola consulta. Si la prueba de rendimiento (T15) no cumple, se añade la tabla de
cierre mantenida por el servicio; el contrato de la API no cambia.

Cada work item guarda `depth` (0 = raíz) para validar RF-78 sin recorrer el árbol:
al mover, `nueva profundidad del subárbol = depth(nuevo padre) + 1 + altura(subárbol)`.

### Integridad en BD (RNF de integridad)
Las reglas «del mismo proyecto» se garantizan con **claves foráneas compuestas**, no solo en el
servicio: `work_items`, `milestones` y `project_phases` tienen `UNIQUE (id, project_id)` y las
referencias usan `(x_id, project_id)`:

- `work_items (parent_id, project_id) → work_items (id, project_id)` (RF-76)
- `milestone_work_items (milestone_id, project_id) → milestones`, `(work_item_id, project_id) → work_items` (RF-51)
- `milestones (phase_id, project_id) → project_phases` (RF-58)
- `key_result_milestones (objective_id, milestone_id) → objective_milestones ON DELETE CASCADE` y
  `(key_result_id, objective_id) → key_results (id, objective_id)` (RF-108, RF-109)

**Ciclos y concurrencia:** los cambios estructurales de un proyecto (mover un work item, mover
el proyecto, crear dependencias `blocks`/`precedes`) toman `pg_advisory_xact_lock` antes de
comprobar ciclos y profundidad, así dos movimientos simultáneos no crean un ciclo entre los dos.
Claves: `hashtext('wi-tree:' || project_id)`, `hashtext('project-tree')` y
`hashtext('deps:' || type)`.

**Contador de work items (RF-62, RF-63):** `UPDATE projects SET wi_counter = wi_counter + 1
WHERE id = $1 RETURNING wi_counter` dentro de la transacción del alta. El bloqueo de fila
serializa las altas del proyecto; un alta fallida deshace el incremento.

### Valores derivados (RF-80 a RF-84)
Columnas guardadas en `work_items`, actualizadas en la misma transacción. Cada nodo guarda
agregados de su subárbol (él incluido):

| Columna | Valor |
|---|---|
| `agg_estimated` | Σ horas estimadas (= `derived_estimated_hours`) |
| `agg_remaining` | Σ horas restantes (= `derived_remaining_hours`) |
| `agg_count` | nº de nodos |
| `agg_ratio_sum` | Σ avance efectivo (100 si cerrado) |
| `agg_est_ratio_sum` | Σ estimadas × avance efectivo |

`derived_done_ratio = agg_est_ratio_sum / agg_estimated` si `agg_estimated > 0`, si no
`agg_ratio_sum / agg_count` (RF-81, RF-82). Tras cualquier cambio se recalculan **solo los
ancestros**, de abajo arriba: `agg(n) = propio(n) + Σ agg(hijos)`. Son ≤ 10 `UPDATE` por índice
de `parent_id`, también para el padre antiguo al mover. Los derivados **no** incrementan
`version` (RF-73), para que editar un padre no choque con cambios de sus hijos.

Como `is_closed` de un estado afecta a derivados y eventos, **no se permite cambiar
`is_closed` de un estado que ya usa algún work item** (`status_closed_locked`).

### Hitos efectivos (RF-52 a RF-57)
Sin guardar la herencia. Consultas:
- **Work items de un hito:** CTE recursiva desde `milestone_work_items` hacia los hijos,
  `GROUP BY id` con `bool_or(direct)`.
- **Hitos de un work item:** CTE de ancestros (incluido él) unida con `milestone_work_items`.
- **Avance del hito:** la misma CTE con `count(*) FILTER (WHERE s.is_closed)` y la suma de las
  horas **propias** (RF-55).
- **Filtro por hito efectivo** en el listado de work items: `id IN (CTE)`.

### Campos personalizados (D4)
Mismo modelo que los atributos de 003: definición en `custom_fields` (más `entity`) y valores
en tablas con clave foránea real, `project_custom_values` y `work_item_custom_values`
(`value jsonb`). La validación por tipo reutiliza `attributes.Normalize` a través de un tipo
común extraído a `internal/fields` (sin cambiar el comportamiento de 003). No hay filtros por
campos personalizados en la spec, así que no hace falta índice GIN.

### Texto enriquecido (RF-70, RF-71)
La descripción se guarda como **documento JSON de TipTap/ProseMirror** (`jsonb`), nunca HTML.
`internal/richtext` lo valida en el servidor con una lista blanca:
- **Nodos:** `doc`, `paragraph`, `heading` (niveles 1–3), `bulletList`, `orderedList`, `listItem`,
  `codeBlock`, `blockquote`, `hardBreak`, `horizontalRule`, `table`, `tableRow`,
  `tableHeader`, `tableCell`, `text`.
- **Marcas:** `bold`, `italic`, `strike`, `code` y `link`, que solo admite `href` con
  `http`, `https` o `mailto`.

Cualquier otro nodo, marca o atributo se rechaza con `invalid_rich_text`. También se rechaza si
el texto pasa de 50 000 caracteres, el JSON de 256 KiB o la anidación de 20 niveles. Se extrae el
texto plano a `description_text` para la búsqueda. El frontend lo pinta con TipTap en solo
lectura, sin `v-html`. La auditoría guarda solo `description_changed: true` (RF-135).

### Control de versión (RF-73, RF-74)
`version int` en `projects`, `milestones`, `work_items`, `objectives` y `key_results`. Toda
modificación (`PATCH` y acciones de estado) lleva `version` y hace
`UPDATE … WHERE id = $1 AND version = $2 … SET version = version + 1`. Con 0 filas y el registro
existente → 409 `version_conflict`, con la versión actual en `details`.

### Eventos de dominio (RF-116 a RF-122): outbox
- Tabla `domain_events` (`id bigint identity`, `event_id uuid`, `type`, `occurred_at`,
  `actor_id`, `project_id`, `subject_type`, `subject_id`, `payload jsonb`). Sin claves
  foráneas: un evento sobrevive al borrado de su work item.
- `events.Emit(ctx, q, Event)` escribe en la transacción del cambio, como `audit.Record`.
  Antes toma `pg_advisory_xact_lock(hashtext('domain_events'))`, para que los ids se confirmen
  en orden y un lector por cursor no se salte un evento de una transacción más lenta. Con el
  volumen esperado (unas decenas de cierres por hora) el bloqueo es despreciable.
- `events.Dispatcher`: goroutine iniciada en `main` (como el hub) que cada segundo lee los
  eventos posteriores al cursor de cada consumidor (`event_consumers(name, last_event_id)`),
  llama al consumidor y avanza el cursor después. Entrega al menos una vez y en orden
  (RF-121). En esta spec no hay consumidores reales; los tests registran uno de prueba.
- Los proyectos plantilla no emiten eventos (RF-35).

### Permisos (RF-123 a RF-127)
Permisos nuevos en la matriz única de `authz`:

```
work:read                                   todos
templates:use                               admin, lead
catalog:manage, templates:manage            admin
projects:create:all | projects:create:team  admin | lead
projects:manage:all | projects:manage:team  admin | lead
projects:manage:owner                       lead, member   (responsable; viewer nunca)
workitems:write:all | workitems:write:team  admin | lead
workitems:write:participant                 lead, member
workitems:delete:own                        lead, member   (autor, sin hijos)
objectives:create:all                       admin          (cualquier alcance)
objectives:create:team                      lead           (alcance equipo/proyecto en su ámbito)
objectives:manage:all | objectives:manage:team  admin | lead
objectives:manage:owner                     lead, member
```

`authz.ProjectRef{TeamID, OwnerID}` y `authz.ObjectiveRef{ScopeTeamID, OwnerID}` con métodos
`CanManageProject`, `CanWriteWorkItems`, `CanDeleteWorkItem`, `CanManageObjective`. El ámbito
del `lead` usa `ScopeTeams` de 003. Un `lead` también es participante donde sea miembro: la
matriz da a `lead` «en su ámbito», pero no quitarle lo que tendría como `member` es la lectura
coherente con RF-58 de 003 (unión de permisos).

**Participante:** el `Principal` gana `ParticipantTeams`, que contiene los equipos del usuario
y todos sus ancestros. Así «el equipo del proyecto es un equipo del usuario o un ancestro de uno»
se comprueba en O(1). El loader de 003 añade esa consulta (CTE ascendente), con la misma caché
de 60 s.

El ámbito de un objetivo: `organizacion` → ninguno; `equipo` → su equipo; `proyecto` → el
equipo del proyecto.

Ligar un hito u objetivo comprueba el permiso del hito u objetivo (RF-127), y además que el
work item sea visible (todos lo son).

### Listados
Squirrel solo para los listados con filtros dinámicos (`projects`, `work_items`, `objectives`).
Texto con `unaccent(lower(...)) LIKE`, como el directorio de 003. El árbol de un proyecto
(RF-130) es una sola consulta plana ordenada por `depth`/`position` con los campos de la
tabla; el frontend arma el árbol.

## Esquema (migración `00003_proyectos.sql`, solo aditiva)

```
methodologies(id, name, name_normalized, position, retired_at, created_at, updated_at)
phase_definitions(id, methodology_id fk, name, position, has_start_gate, has_finish_gate, retired_at)
work_item_types(id, name, name_normalized, color, position, retired_at, …)
statuses(id, name, name_normalized, color, is_closed, is_default, position, retired_at, …)
    unique (is_default) where is_default; check (not (is_default and is_closed))
priorities(id, name, name_normalized, color, position, is_default, retired_at, …)
    unique (is_default) where is_default
workflow_transitions(id, type_id fk, from_status_id fk, to_status_id fk,
    role text null check (role in admin|lead|member|viewer),
    unique nulls not distinct (type_id, from_status_id, to_status_id, role))
-- índices únicos parciales de nombre activo en cada catálogo (RF-8)

projects(id, parent_id fk, template_id fk, team_id fk teams, owner_id fk users,
    methodology_id fk null, identifier text unique check (~ '^[A-Z][A-Z0-9]{1,9}$'),
    name, description jsonb, description_text, is_template, lifecycle check (activo|cerrado|archivado),
    health check (en_curso|en_riesgo|fuera_de_curso), health_explanation,
    start_date date, end_date date check (end_date >= start_date), depth smallint,
    wi_counter int, version int, created_at, updated_at)
project_work_item_types(project_id, type_id, pk)
custom_fields(id, entity check (project|work_item), name, name_normalized, type, required,
    options jsonb, retired_at, …)
custom_field_work_item_types(field_id, type_id, pk)            -- vacío = todos los tipos (RF-14)
project_custom_fields(project_id, field_id, pk)                -- activados (RF-15)
project_custom_values(project_id, field_id, value jsonb, pk)
work_item_custom_values(work_item_id fk on delete cascade, field_id, value jsonb, pk)
project_phases(id, project_id, definition_id, position, start_date, finish_date,
    start_gate_passed_at, start_gate_passed_by, finish_gate_passed_at, finish_gate_passed_by,
    unique (id, project_id))

milestones(id, project_id, phase_id null, name, description jsonb, description_text,
    due_date, completed_on date, status check (abierto|cumplido|cancelado), position, version, …,
    unique (id, project_id), fk (phase_id, project_id) → project_phases,
    check ((status = 'cumplido') = (completed_on is not null)))
work_items(id, project_id, type_id, status_id, priority_id, parent_id null, sequence_number,
    title check (length between 1 and 255), description jsonb, description_text,
    author_id, assignee_id null, start_date, due_date check (due_date >= start_date),
    estimated_hours numeric(7,2) check 0..10000, remaining_hours numeric(7,2) check 0..10000,
    done_ratio smallint check 0..100, depth smallint check 0..10,
    agg_estimated, agg_remaining, agg_count, agg_ratio_sum, agg_est_ratio_sum,
    version, closed_at, created_at, updated_at,
    unique (project_id, sequence_number), unique (id, project_id),
    fk (parent_id, project_id) → work_items (id, project_id))
milestone_work_items(milestone_id, work_item_id, project_id, pk (milestone_id, work_item_id),
    fks compuestas, on delete cascade desde work_items)
work_item_dependencies(id, from_id, to_id, type check (blocks|precedes|relates|duplicates),
    lag_days smallint check -365..365 (solo precedes), created_by, created_at,
    unique (from_id, to_id, type), check (from_id <> to_id), on delete cascade)

objectives(id, title, description jsonb, description_text, owner_id,
    scope check (organizacion|equipo|proyecto), scope_team_id null, scope_project_id null,
    period_start, period_end, status check (borrador|activo|cerrado|cancelado), version, …,
    check de coherencia alcance ↔ columna)
key_results(id, objective_id, title, unit, start_value numeric, target_value numeric,
    current_value numeric, calc_mode check (manual|derivado), position, version,
    unique (id, objective_id), check (start_value <> target_value))
objective_projects(objective_id, project_id, pk)
objective_milestones(objective_id, milestone_id, pk)
key_result_milestones(key_result_id, objective_id, milestone_id, pk (key_result_id, milestone_id),
    fk (objective_id, milestone_id) → objective_milestones on delete cascade,
    fk (key_result_id, objective_id) → key_results (id, objective_id))
key_result_work_items(key_result_id, work_item_id fk on delete cascade, pk)

domain_events(id bigint identity pk, event_id uuid unique, type, occurred_at, actor_id,
    project_id, subject_type, subject_id, payload jsonb)
event_consumers(name text pk, last_event_id bigint)
```

**Índices:** `work_items (project_id, parent_id)`, `(parent_id)`, `(assignee_id)`,
`(project_id, status_id)`; `projects (parent_id)`, `(team_id)`; `milestone_work_items
(work_item_id)`; `work_item_dependencies (to_id)`; `objective_milestones (milestone_id)`;
`objective_projects (project_id)`; `key_result_work_items (work_item_id)`.

**Datos iniciales (RF-11)** en la misma migración:
- **Estados:** Nuevo (por defecto), En progreso, En revisión, Resuelto (cerrado), Cerrado (cerrado), Rechazado (cerrado).
- **Tipos:** Épica, Actividad, Subactividad, Story, Tarea, Bug.
- **Prioridades:** Baja, Normal (por defecto), Alta, Urgente.
- **Metodologías:**
  - Scrum, con las fases Preparación y Ejecución por sprints, sin gates.
  - PRINCE2, con las fases Puesta en marcha, Inicio, Etapas de entrega y Cierre, cada una con gates de entrada y salida.

Son filas normales, editables y retirables.

## Backend

### Paquetes

```
internal/
  fields/        tipos de campo y Normalize compartidos (extraídos de attributes, sin cambios de 003)
  richtext/      validación del documento TipTap y extracción de texto
  events/        Emit (outbox) + Dispatcher
  catalog/       metodologías, fases, tipos, estados, prioridades, transiciones
  customfields/  definición de campos y aplicación de valores a proyectos y work items
  projects/      proyectos, jerarquía, tipos y campos activos, fases, plantillas, listado
  milestones/    hitos, vínculos con work items, efectivos y avance
  workitems/     work items, numeración, composición, derivados, flujos, dependencias, listado y árbol
  objectives/    objetivos, resultados clave, vínculos y avance
```

Mismo patrón que 003: `handler.go` / `service.go` / `types.go`, con `db.Store` directo (ver
desviaciones de 003). Cada mutación: `InTx` → comprobar permiso con datos leídos **dentro** de
la transacción (`SELECT … FOR UPDATE` del registro que se cambia) → cambio → `audit.Record` →
`events.Emit` si toca.

### API

Prefijo `/api`, JSON `snake_case`, errores con el formato común. Toda ruta requiere sesión; las
de escritura, CSRF. Las fichas devuelven un objeto `can` con las acciones permitidas al que
consulta (RF-137) y `version`.

| Método y ruta | Permiso | RF |
|---|---|---|
| `GET /api/catalog` (todo el catálogo, para formularios) | `work:read` | RF-1 a RF-11 |
| `POST/PATCH /api/methodologies[/{id}]` · `/retire` · `/restore` · `POST/PATCH /api/methodologies/{id}/phases[/{pid}]` | `catalog:manage` | RF-1, RF-2, RF-8, RF-9 |
| `POST/PATCH /api/work-item-types|statuses|priorities[/{id}]` · `/retire` · `/restore` | `catalog:manage` | RF-3 a RF-10 |
| `GET/PUT /api/work-item-types/{id}/transitions` | `catalog:manage` (GET: `work:read`) | RF-6 |
| `GET/POST /api/custom-fields` · `PATCH /api/custom-fields/{id}` · `/retire` · `/restore` | `catalog:manage` (GET: `work:read`) | RF-12 a RF-17 |
| `GET /api/projects` (cada fila con `can`) · `POST /api/projects` | `work:read` · `projects:create:*` | RF-18 a RF-28, RF-128, RF-141 |
| `GET/PATCH /api/projects/{id}` · `POST /api/projects/{id}/close|reopen|archive|unarchive` | `work:read` · gestor | RF-21 a RF-31 |
| `GET /api/projects/{id}/subtree` | `work:read` | RF-28 |
| `PUT /api/projects/{id}/types` · `PUT /api/projects/{id}/custom-fields` | gestor | RF-15, RF-32, RF-33 |
| `POST /api/projects/{id}/template` · `DELETE …/template` | `templates:manage` | RF-34, RF-35 |
| `GET /api/templates` · `POST /api/projects/from-template` | `templates:use` + `projects:create:*` | RF-36 a RF-39 |
| `GET /api/projects/{id}/phases` · `PATCH /api/phases/{id}` · `POST /api/phases/{id}/gates/{start|finish}` | `work:read` · gestor | RF-40 a RF-43 |
| `GET/POST /api/projects/{id}/milestones` · `GET/PATCH/DELETE /api/milestones/{id}` · `POST /api/milestones/{id}/complete|reopen|cancel` | `work:read` · gestor | RF-44 a RF-49, RF-58 a RF-60 |
| `GET /api/milestones/{id}/work-items` · `POST/DELETE /api/milestones/{id}/work-items[/{wid}]` | `work:read` · gestor | RF-50 a RF-56 |
| `GET /api/projects/{id}/work-items` · `GET /api/projects/{id}/tree` · `POST /api/projects/{id}/work-items` | `work:read` · participante | RF-61 a RF-72, RF-129, RF-130 |
| `GET /api/work-items/{id}` · `GET /api/work-items/by-key/{KEY-N}` · `PATCH /api/work-items/{id}` · `DELETE` | `work:read` · participante · borrar | RF-61 a RF-89, RF-133 |
| `GET /api/work-items/{id}/ancestors|descendants|milestones` | `work:read` | RF-53, RF-79 |
| `POST /api/work-items/{id}/dependencies` · `DELETE /api/dependencies/{id}` | participante (origen) | RF-90 a RF-96 |
| `GET/POST /api/objectives` · `GET/PATCH /api/objectives/{id}` · `POST /api/objectives/{id}/activate|close|cancel` | `work:read` · `objectives:*` | RF-97 a RF-100, RF-131 |
| `POST/PATCH/DELETE /api/objectives/{id}/key-results[/{kid}]` | gestor del objetivo | RF-101 a RF-103, RF-111 |
| `POST/DELETE /api/objectives/{id}/projects|milestones[/{xid}]` · `POST/DELETE /api/key-results/{id}/work-items|milestones[/{xid}]` | gestor del objetivo | RF-104 a RF-110, RF-127 |
| `GET /api/events` | `audit:read` | RF-116 a RF-122 (consulta para la demo) |

El cambio de estado va en el `PATCH` del work item (`status_id` + `version`); el servicio aplica
el flujo (RF-86 a RF-88), el bloqueo (RF-95), RF-84 y los eventos.

**Códigos de error nuevos:** `catalog_name_conflict`, `catalog_default_required`,
`status_closed_locked`, `identifier_invalid`, `identifier_conflict`, `invalid_date_range`,
`invalid_owner`, `invalid_assignee`, `project_cycle`, `project_too_deep`, `project_closed`,
`project_archived`, `type_not_enabled`, `type_in_use`, `custom_field_required`,
`custom_field_invalid`, `custom_field_not_enabled`, `not_a_template`, `cross_project`,
`work_item_cycle`, `work_item_too_deep`, `has_children`, `transition_not_allowed`,
`blocked_by_open_item`, `dependency_conflict`, `dependency_cycle`, `dependency_hierarchy`,
`milestone_in_use`, `key_result_required`, `key_result_target_equal`, `key_result_derived`,
`milestone_not_in_objective`, `version_conflict`, `invalid_rich_text`, `invalid_filter`
(existente), `field_out_of_range`.

## Frontend

- `api/work.ts`: clientes y tipos; `types/api.ts` amplía.
- `features/`:
  - `catalog/`: administración de catálogo.
  - `projects/`: listado, alta desde plantilla y ficha con pestañas Resumen, Fases, Hitos y Work items.
  - `milestones/`: ficha con avance y work items efectivos, distinguiendo directos de heredados.
  - `workitems/`: árbol, ficha, formulario, transiciones y dependencias.
  - `objectives/`: listado y ficha con resultados clave, hitos contribuyentes y proyectos.
  - `richtext/`: `RichTextEditor.vue` y `RichTextView.vue` sobre TipTap, con las mismas extensiones que la lista blanca del servidor.
- **Árbol de 5 000 work items:** la API devuelve una lista plana; el frontend la indexa por
  padre y solo pinta los nodos expandidos (por defecto, el primer nivel). Sin virtualización
  ni dependencias extra; si el árbol expandido pesa, se añade `@tanstack/vue-virtual`.
- Acciones visibles según `can` (RF-137); errores por código (RF-138); textos en `es.json`.
- **Edición en tabla (RF-139 a RF-144):** `components/data-table/` con `GridTable` (marco y
  flechas entre celdas), `EditableCell` (texto, número, fecha, selector, casilla y color; las
  opciones de un selector pueden cargarse al editar, como los estados permitidos) y
  `QuickAddRow` (alta con Enter). Cada pantalla pasa a la celda una función que lee el
  elemento, cambia el campo y hace el `PATCH` (`features/workitems/patch.ts` para work items).
  Se usan en proyectos, hitos, árbol de work items, catálogo, directorio y equipos.
- Conflicto de versión: aviso con opción de recargar el elemento.
- Rutas:
  - `/projects`, `/projects/new`, `/projects/:id`
  - `/milestones/:id`
  - `/work-items/:key`, que acepta `KAI-42`
  - `/objectives`, `/objectives/:id`
  - `/admin/catalog`

  Todas se añaden al menú del layout según los permisos.

## Tests

- **Integración** (`dbtest` + `apitest`) por RF, igual que 003.
- **Concurrencia:**
  - 20 altas simultáneas en un proyecto dan 20 números distintos y consecutivos (RF-63).
  - Dos `PATCH` con la misma versión: uno da 200 y el otro 409 (RF-73).
  - Dos movimientos cruzados simultáneos (A bajo B y B bajo A): uno se rechaza (RF-77).
  - Dependencias `blocks` cruzadas en paralelo: no dejan ciclo (RF-94).
- **Derivados:** test de tabla con árboles pequeños, comparando con un cálculo de referencia en
  Go después de cada operación (alta, edición, cierre, mover y borrar).
- **Eventos:** emisión y no emisión según RF-116 a RF-118. Si la transacción se deshace, no queda
  evento. El dispatcher entrega en orden y vuelve a entregar si el consumidor falla.
- **Texto enriquecido:** tabla de documentos válidos e inválidos (`<script>`, `javascript:`,
  nodos desconocidos, atributos extra, tamaño y anidación).
- **Matriz de permisos (RF-125):** amplía `apitest/matrix_test.go` con cada operación nueva para
  `admin`, `lead` dentro y fuera de ámbito, `member` participante y no participante, `member`
  responsable y `viewer`.
- **Rendimiento (T15):** genera con `COPY` el tamaño de diseño:
  - 500 proyectos;
  - 50 000 work items, 5 000 de ellos en un proyecto, con profundidad 10;
  - un hito con 1 000 efectivos;
  - 200 objetivos con 10 resultados clave.

  Mide cada umbral de los RNF con la mediana de 5 ejecuciones. Corre en `make test`; si tarda
  más de 30 s, pasa a `make test-perf`.
- **Frontend (Vitest):**
  - armado del árbol a partir de la lista plana;
  - lista blanca del editor alineada con la del servidor, con fixtures compartidos;
  - visibilidad de acciones según `can`;
  - traducción de los códigos nuevos y claves i18n.

## Riesgos y limitaciones

- **Bloqueo global del outbox:** serializa las transacciones que emiten eventos. Es suficiente
  para el volumen previsto; con más carga se cambiaría a cursores por `xid8`/`pg_snapshot`.
- **Consulta recursiva frente a tabla de cierre:** la decide la prueba de rendimiento (T15).
- **Derivados guardados:** un fallo de lógica dejaría valores incoherentes. El test de
  referencia lo cubre; además, una función SQL `recompute_work_item_aggregates(project_id)`
  permite reconstruirlos y la usa el test.
- **Fusión:** esta rama depende de la PR #7. Se abre su PR después de fusionar la 003, o con
  base `feat/003-usuarios`.

## Tareas
Ver [tasks.md](tasks.md).

## Verificación
- `make test` en verde (backend con PostgreSQL y prueba de rendimiento, frontend con Vitest) y
  `npm run build`.
- Prueba de humo en navegador de los pasos de la demo.
- Demo manual de los criterios de finalización de la spec, pasos 1 a 14.
