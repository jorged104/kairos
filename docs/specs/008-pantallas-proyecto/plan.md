# Plan 008 — Pantallas de proyecto

Plan de implementación de [spec.md](spec.md) (aprobada 2026-11-02). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias nuevas
Ninguna. El arrastre del kanban y del cronograma usa eventos de puntero y la API de arrastre
nativa del navegador; VueDraggablePlus (en el stack de `AGENTS.md`) no está instalado y no hace falta.

## Esquema (migración `00008_pantallas_proyecto.sql`, solo aditiva)
- **`project_members`:** `(project_id, user_id)` PK, `role text CHECK IN ('pm','tech_lead','member','observer')`,
  `created_at`. Solo los papeles puestos expresamente; el responsable (PM) y los participantes por
  equipo (miembro) son implícitos (RF-34 a RF-36). Índice por `user_id`.
- **`users.weekly_capacity int NULL CHECK 0..1000`** (RF-46).
- **`project_views`:** `id`, `project_id` (CASCADE), `owner_id` (null = compartida), `name`,
  `name_normalized`, `config jsonb` (filtros, orden, agrupación), fechas. Índices únicos:
  `(project_id, owner_id, name_normalized)` para personales y `(project_id, name_normalized) WHERE owner_id IS NULL`
  para compartidas; el cruce personal/compartida lo comprueba el servicio (RF-29).
- **`project_column_limits`:** `(project_id, status_id)` PK, `wip_limit int CHECK 1..999` (RF-66).
- **`project_progress_daily`:** `(project_id, day date)` PK, `progress smallint`, `expected smallint NULL` (RF-12).

## Autorización (`authz`)
- `ProjectRef` gana `ID`; `projects.Ref` y el listado lo rellenan.
- `Principal.ProjectRoles map[uuid.UUID]ProjectRole`, cargado por `PrincipalLoader` con los papeles
  expresos del usuario (misma caché de 60 s, RF-44). `projects` invalida la caché del usuario
  afectado al cambiar un papel (hook `OnMembershipChanged` cableado en `server/app.go`).
- `ProjectRoleOf(pr)`: responsable → PM; papel expreso; participante → miembro; si no, ninguno.
- `CanManageProject` añade `PM && Has(ProjectsManageOwner)` (un `viewer` global nunca gestiona).
- `CanWriteWorkItems`: la rama de participante pasa a «papel PM, líder técnico o miembro y
  `Has(WorkItemsWriteParticipant)`»; un observador no escribe (RF-42). Las ramas globales
  (`all`, `team` en su ámbito) quedan delante (RF-43).
- `CanDeleteWorkItem` añade «líder técnico y `Has(WorkItemsDeleteOwn)`» (RF-40).
- Sin permisos nuevos en la matriz: los papeles se apoyan en los existentes. El test de matriz gana
  filas para PM, líder técnico y observador por proyecto.

## Backend
- **`projects/members.go`** (`GET/POST /projects/{id}/members`, `PATCH/DELETE /projects/{id}/members/{userId}`):
  lista con papel efectivo, origen (`owner`, `team`, `explicit`), capacidad, carga en el proyecto,
  carga total y tareas abiertas (una consulta agregada por `work_item_assignees` + `statuses`).
  Validaciones RF-35, RF-37, RF-38; auditoría `project.member_*` (RF-45). Solo `CanManageProject`.
- **`projects/suggestions.go`** (`GET /projects/{id}/assignment-suggestions`): tareas abiertas sin
  asignados y hasta 2 sugerencias por tarea según RF-52 (miembros con papel de trabajo, activos,
  que comparten etiqueta; orden por carga/capacidad, sin capacidad al final).
- **`users`:** `PATCH /users/{id}/capacity {capacity: int|null}` con `CanActOn(UsersUpdateAll, UsersUpdateTeam)`
  (RF-47), auditado; `capacity` en el detalle del usuario.
- **Filtros (`workitems/filters.go`)**: un solo constructor Squirrel para el listado y el árbol.
  Parámetros repetibles (o dentro de cada uno, y entre ellos): `status_id`, `type_id`,
  `priority_id`, `assignee_id` (`none`, `me`), `label_id`, `size_id`, `milestone_id` (efectivo);
  `closed`, `due_before`, `due_after`, `overdue`, `blocked`, `q`. `sort` = campo con `-` para
  descendente (RF-57).
- **Árbol filtrado:** `GET /projects/{id}/tree` acepta los mismos filtros (sin paginar). `Node` gana
  `start_date`, `blocked` y `milestone_ids` (efectivos) para el kanban y el cronograma.
- **Vistas (`projects/views.go`)**: CRUD en `/projects/{id}/views` y `/views/{id}`; `config` validado
  (campos y operadores conocidos, ≤ 30 filtros, ≤ 50 valores por filtro); permisos RF-28; límites
  RF-30; `GET` devuelve las visibles con `count` calculado con los filtros (RF-26).
- **Resumen (`projects/overview.go`, `GET /projects/{id}/overview?today=`)**: avance ponderado del
  proyecto (`sum(weight·ratio)/sum(weight)` con cerradas = 100), esperado y diferencia, conteo por
  estado, vencidas, bloqueadas, próximo hito, riesgos (máx. 10, RF-15) con el umbral ±10 de la spec
  007, carga de los 5 primeros y la serie diaria.
- **Registro diario (`projects/progress_job.go`)**: al arrancar y cada hora, `INSERT … ON CONFLICT
  (project_id, day) DO UPDATE` del avance de los proyectos `activo` (RF-12). Varias instancias
  escriben lo mismo, sin carreras dañinas.
- **Límites del kanban:** `PUT /projects/{id}/column-limits/{statusId} {limit|null}` (RF-66) y
  `column_limits` en la ficha del proyecto.
- **Ficha del proyecto:** `can` gana `manage_team` (RF-4, RF-25) y `my_role`.

## Frontend
- **`ProjectPage`:** pestañas Resumen, Tabla, Kanban, Cronograma, Equipo (solo `can.manage_team`),
  Hitos, Fases y Datos (RF-1, RF-4). `?tab=`, `?view=` y los filtros en la URL (RF-3).
- **Filtros y vistas (`features/views/`)**: `useProjectFilters` (estado ↔ URL ↔ consulta), barra de
  vistas con contadores, chips, «+ Filtro» (campo, valores), «Cambios sin guardar», guardar y
  compartir; última vista en `localStorage` por proyecto (RF-32).
- **Resumen (`features/projects/ProjectOverview.vue`)**: tarjetas, gráfica SVG propia (sin
  librería), hitos, riesgos y carga.
- **Tabla:** `ProjectWorkItems` usa el árbol filtrado; orden y agrupación (RF-55 a RF-59).
- **Kanban (`features/kanban/`)**: columnas, tarjetas, arrastre nativo con columnas permitidas
  calculadas de `allowed_status_ids` (se piden al empezar a arrastrar), «Mover a», carriles,
  límites, alta por columna (RF-60 a RF-69).
- **Cronograma (`features/timeline/`)**: rejilla con escala, barras posicionadas por fecha,
  arrastre por puntero (mover y extremos) con fechas en días enteros, «Sin fechas», dependencias
  con SVG (RF-70 a RF-79).
- **Equipo (`features/projects/ProjectTeam.vue`)**: miembros, papeles, capacidad (si se puede
  editar), carga, por asignar con sugerencias (RF-33 a RF-54).

## Tests
- **apitest:** papeles y permisos (matriz por papel, observador participante, global no recortado,
  responsable PM fijo, viewer solo observador), miembros (alta, baja, papel, auditoría), capacidad,
  carga y sugerencias, filtros combinados, árbol filtrado, vistas (permisos, conflictos, límites,
  conteos, valor retirado), resumen (avance, esperado, riesgos ordenados, sin plazo), registro
  diario, límites del kanban; rendimiento (2 000 tareas).
- **Vitest:** pestañas y URL, filtros y vistas, resumen, tabla agrupada, kanban (arrastre y «Mover
  a», rechazo), cronograma (mover y extremos), equipo (sugerencia y papel).
