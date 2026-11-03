# Plan 009 — Plan, línea base y semáforo

Plan de implementación de [spec.md](spec.md) (aprobada 2026-11-03). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias nuevas
Ninguna.

## Esquema (migración `00009_linea_base.sql`, solo aditiva)
- **`project_baselines`:** `id`, `project_id` (CASCADE), `version int`, `reason text NULL` (1..500), `created_by` (users), `created_at`. Único `(project_id, version)`.
- **`project_baseline_items`:** `(baseline_id, work_item_id)` PK, ambos CASCADE, `start_date date NULL`, `due_date date NULL`. Borrar una tarea la saca de todas las líneas base (caso límite).
- **`schedule_settings`:** una sola fila (`id boolean PK DEFAULT true CHECK (id)`).
  - Umbrales `upper`, `risk` y `late`, de tipo `numeric(4,1)`, con valores por defecto 5, −5 y −15, y `CHECK (upper > risk AND risk > late)`.
  - `name_ahead`, `name_on_track`, `name_at_risk` y `name_late`: texto que puede ser `NULL`. `NULL` es el nombre por defecto de `es.json`, así los textos por defecto siguen saliendo del catálogo de traducciones.
  - `color_*`: el nombre de un color de la paleta cerrada (`blue`, `green`, `amber`, `red`, `orange`, `violet`, `teal`, `slate`). Por defecto `blue`, `green`, `amber` y `red`.
  - `version`, `updated_at`.
  - La migración inserta la fila.

## Cálculo (`internal/schedule`, funciones puras con tests de tabla)
- **Datos de entrada:**
  - `Item` lleva: id, padre, peso, ratio propio (100 si cerrada), fechas actuales y fechas de la línea base, y si está en ella.
  - `Planned(it)` devuelve las fechas de la línea base si la tarea está en ella; si no, las actuales (RF-9, RF-10).
- **Fracción de una tarea:** `Fraction(start, due, today)` aplica RF-14 a RF-18, con el mismo conteo de días que `projects.days`.
- **Proyecto y hito:** `Aggregate(items, today)` devuelve `plan`, `real` y `has_plan` (RF-20, RF-21).
  - El plan es Σ peso × fracción / Σ peso, sobre las tareas con fin planificado.
  - El real es Σ peso × ratio / Σ peso, sobre todas, como `ProjectProgress`, pero con un decimal.
  - Si todas pesan 0, se usa la media simple, como el avance derivado.
  - Las tareas de peso 0 no aportan.
  - El resultado se recorta a 0..100 (RF-22).
- **Subárbol de una tarea:** `SubtreePlans(items, today)` calcula el mismo agregado sobre la tarea y sus descendientes (RF-19). Es la misma forma que `agg_weight_ratio_sum`.
- **Semáforo:** `Classify(gap, settings)` asigna el nivel `ahead`, `on_track`, `at_risk` o `late` (RF-27 a RF-31).
- **Salida común:** `Figures{HasPlan, Plan, Real, Gap, Level}`, que devuelven el proyecto, el hito y la tarea (RF-35).
- **Consultas sqlc:**
  - `ProjectPlanItems(project_id)`: las tareas del proyecto con su ratio, las fechas actuales y un `LEFT JOIN` a los ítems de la línea base más reciente.
  - `ProjectMilestoneItemPairs(project_id)`: los pares hito → tarea efectiva, con el mismo `WITH RECURSIVE` que `MilestoneProgress`.

## Configuración del semáforo
- **Lectura:** `GET /api/schedule-settings`, con el permiso `work:read`.
- **Escritura:** `PUT /api/schedule-settings`, con `catalog:manage`. Es el permiso de administración del catálogo, que solo tiene el admin, así que no hay permiso nuevo.
  - Se valida RF-37 y RF-38 (umbrales), RF-39 (nombres de 1 a 30 caracteres, o `null` para el nombre por defecto) y RF-40 (color de la paleta).
  - Lleva `version` y responde `version_conflict` si cambió.
  - Se audita con `schedule_settings.updated`, guardando el antes y el después (RF-50).
- **Uso en el cálculo:** se lee en cada cálculo, sin caché (RF-41).

## Línea base (`internal/projects/baseline.go`)
- **Fijar:** `POST /api/projects/{id}/baseline` con `{reason}`.
  - Se carga el proyecto con `teamChange`, que exige `CanManageProject` y `Writable` (RF-7, RF-8).
  - El motivo es obligatorio si ya hay una versión (RF-2).
  - En la misma transacción se calcula la siguiente versión (RF-3), se copian las fechas de las tareas (RF-1) y se audita `project.baseline_set` con la versión y el motivo (RF-49).
  - Responde 201 con la versión.
- **Consultar:** `GET /api/projects/{id}/schedule?today=`, para quien ve el proyecto. Devuelve:
  - `baseline`: la versión vigente o `null`, con su versión, fecha, autor y motivo (RF-5);
  - `versions`: las versiones anteriores (RF-6);
  - `items`: por tarea, `figures`, `in_baseline`, `base_start`, `base_due` y `shift_days`, que es la diferencia en días entre el fin actual y el de la línea base, y si no la hay, entre los inicios (RF-11, RF-12).
  - La usan el cronograma (RF-13, RF-48) y la ficha de la tarea.

## Sustitución del esperado lineal
- **Resumen (`projects/overview.go`):**
  - `Overview` gana `figures` (RF-44) y `plan_series` (RF-46);
  - `MilestoneLine` gana `figures` (RF-45);
  - `has_plan`, `expected` y `deviation` siguen, como el plan redondeado y la brecha, para no romper la versión anterior de la API durante la previsualización;
  - el respaldo lineal con las fechas del proyecto cubre RF-23; RF-24 y RF-25 dan `has_plan=false`.
- **Riesgos:** pasan a «retrasada» las tareas abiertas con nivel `at_risk` o `late` (RF-47). `Risk` gana `level` y `gap`; `deviation` se queda con el valor de la brecha.
- **Serie del plan:** el plan de cada día entre el inicio de la gráfica (el primer día registrado o el inicio del proyecto) y el fin. Se limita a 400 puntos con un paso de N días cuando el rango es mayor.
- **Métricas de la tarea (`workitems/metrics.go`):**
  - `Schedule` pasa a calcularse con `SubtreePlans` y el semáforo (RF-43);
  - `state` toma los valores `ahead`, `on_track`, `at_risk` o `late`;
  - `Schedule` gana `plan`, `gap`, `in_baseline`, `base_start`, `base_due` y `shift_days`;
  - `expected` y `deviation` se mantienen redondeados;
  - desaparece `OnTrackMargin`.

## Frontend
- **API y tipos:**
  - `api/schedule.ts`: `getScheduleSettings`, `updateScheduleSettings`, `getProjectSchedule`, `setBaseline`;
  - tipos `Figures`, `Level` y `ScheduleSettings`.
- **`features/schedule/useLevels.ts`:**
  - devuelve el nombre de cada nivel: el configurado o `t('schedule.levels.<level>')`;
  - mapea el color de la paleta a clases de Tailwind para tema claro y oscuro, con clases fijas en un mapa para que Tailwind las genere.
- **`features/schedule/LevelBadge.vue`:** la pastilla del semáforo, con plan, real y brecha.
- **Resumen (`ProjectOverview.vue`):**
  - la tarjeta de avance muestra el semáforo con plan, real y brecha;
  - un bloque «Línea base» enseña la versión, la fecha, el autor y el motivo, el botón «Fijar línea base» o «Volver a fijar» (este último pide el motivo) y la lista de versiones;
  - los hitos muestran plan, brecha y semáforo;
  - la gráfica dibuja `plan_series`;
  - los riesgos muestran el nivel.
- **Cronograma (`ProjectTimeline.vue`):**
  - usa `getProjectSchedule`;
  - dibuja la barra de la línea base como sombra punteada detrás de la barra actual (RF-13);
  - el color de la barra viene del semáforo: rojo si está vencida o bloqueada, como hoy (RF-71); si no, el color de su nivel cuando es `at_risk` o `late` (RF-48);
  - desaparece el cálculo ±10 del cliente.
- **Ficha (`MetricsSection.vue`):** el semáforo, el plan y la brecha, la marca «Fuera de línea base» y las fechas de la línea base con su diferencia (RF-11, RF-12).
- **Catálogo (`CatalogPage.vue`):** sección «Semáforo» (`features/catalog/ScheduleSettingsAdmin.vue`) con los tres umbrales, los nombres y los colores de la paleta.
- **Textos:** todos en `es.json` (`schedule.*`).

## Tests
- **Unitarios Go (`schedule`):** fracción en cada caso, agregado con pesos 0, recorte, subárbol, clasificación en los bordes exactos de los umbrales.
- **apitest:**
  - fijar la línea base: primera sin motivo, la segunda sin motivo da 400, versiones y auditoría;
  - un miembro recibe 403 y un proyecto cerrado da error;
  - mover una tarea no cambia el plan;
  - una tarea nueva sale fuera de la línea base;
  - la configuración del semáforo: validaciones, `version_conflict`, auditoría y efecto inmediato en el Resumen;
  - Resumen, hitos y métricas con plan y nivel.
- **Matriz:** filas «fijar línea base» (gestores, 201) y «configurar semáforo» (solo admin).
- **Vitest:** pastilla y nombres configurados, Resumen con la línea base, cronograma con la sombra, administración del semáforo.
