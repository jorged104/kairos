# Tareas 004 — Proyectos, metodologías, hitos, work items y objetivos

Derivadas de [plan.md](plan.md) para [spec.md](spec.md), en orden de dependencia.
Un commit por tarea en `feat/004-proyectos`; ninguna se marca sin su verificación en verde.
Las tareas marcadas **(tú)** necesitan una comprobación manual tuya.

- [x] **T1 — Esquema, datos iniciales y queries** (`00003_proyectos.sql`, `queries/*.sql`, `make sqlc`)
  - Depende de: —
  - RF: soporte de todos; RF-11
  - Hecho cuando: la migración sube y baja en limpio; sqlc genera sin errores; tests de BD de las claves compuestas (padre, hito y fase de otro proyecto rechazados por la BD), de los `CHECK` (identificador, fechas, horas, avance, hito cumplido ↔ fecha) y de que solo hay un estado y una prioridad por defecto; los datos iniciales existen.

- [x] **T2 — Permisos y participante** (`internal/authz`, loader de `internal/auth`)
  - Depende de: T1
  - RF: RF-123, RF-124, RF-126
  - Hecho cuando: tests unitarios de los permisos nuevos por rol, de `CanManageProject`, `CanWriteWorkItems`, `CanDeleteWorkItem` y `CanManageObjective` (ámbito, responsable, participante por equipo ancestro, `viewer` nunca); el loader carga `ParticipantTeams`.

- [x] **T3 — Piezas comunes: texto enriquecido, versión y outbox** (`internal/richtext`, `internal/events`, `internal/fields`)
  - Depende de: T1
  - RF: RF-70, RF-71, RF-116 a RF-122 (infraestructura), RF-13
  - Hecho cuando: tests de la lista blanca (válidos e inválidos, límites); `Emit` en tx deshecha no deja evento; el dispatcher entrega en orden, reintenta si el consumidor falla y no se salta eventos de transacciones lentas; `attributes` sigue pasando sus tests tras extraer `fields`.

- [x] **T4 — Catálogo** (`internal/catalog`)
  - Depende de: T2, T3
  - RF: RF-1 a RF-11
  - Hecho cuando: tests de alta, duplicado, retirada con uso, retirada del por defecto, cambio de estado por defecto, transiciones por tipo y rol, y `status_closed_locked`.

- [x] **T5 — Campos personalizados** (`internal/customfields`)
  - Depende de: T4
  - RF: RF-12 a RF-17
  - Hecho cuando: tests de cada tipo, límite por tipo de work item, activación por proyecto, obligatorios y retirada, reutilizando las reglas de 003.

- [x] **T6 — Proyectos** (`internal/projects`: alta, edición, jerarquía, ciclo de vida, tipos y campos activos, fases, listado)
  - Depende de: T5
  - RF: RF-18 a RF-33, RF-40 a RF-43, RF-128
  - Hecho cuando: tests de cada RF, incluidos identificador inmutable y en conflicto con archivados, ciclo y profundidad 5, responsable inválido, proyecto cerrado y archivado de solo lectura, fases instanciadas de la metodología, subárbol en una consulta y filtros del listado (equipo con subequipos).

- [x] **T7 — Work items** (`internal/workitems`: alta, edición, numeración, asignación, versión, borrado)
  - Depende de: T5, T6
  - RF: RF-61 a RF-74, RF-85, RF-129, RF-133
  - Hecho cuando: tests de cada RF, incluidos 20 altas concurrentes con números distintos, conflicto de versión con dos `PATCH`, asignado inválido, tipo no activo, descripción saneada, búsqueda por `KAI-42` y filtros del listado (hito efectivo incluido).

- [x] **T8 — Composición y derivados** (`internal/workitems`)
  - Depende de: T7
  - RF: RF-75 a RF-85, RF-130
  - Hecho cuando: tests de padre de otro proyecto, ciclo (también con movimientos concurrentes cruzados), profundidad 10, ancestros y descendientes en una consulta, árbol del proyecto; test de referencia de derivados tras alta, edición, cierre, mover y borrar.

- [x] **T9 — Estados, flujos y eventos** (`internal/workitems` + `events`)
  - Depende de: T8, T4
  - RF: RF-84, RF-86 a RF-89, RF-95, RF-116 a RF-118, RF-120 a RF-122
  - Hecho cuando: tests de transición permitida y rechazada por tipo y rol, tipo sin transiciones, admin que fuerza, transiciones posibles en la ficha, cierre bloqueado por `blocks`, y eventos al cerrar, reabrir y entre cerrados (ninguno).

- [x] **T10 — Dependencias** (`internal/workitems`)
  - Depende de: T8
  - RF: RF-90 a RF-96
  - Hecho cuando: tests de los cuatro tipos, entre proyectos, consigo mismo, con ancestro o descendiente, duplicado, ciclo en `blocks`/`precedes` (también concurrente), desfase fuera de rango e incumplimiento de fechas indicado.

- [ ] **T11 — Hitos** (`internal/milestones`)
  - Depende de: T6, T7
  - RF: RF-44 a RF-60
  - Hecho cuando: tests de estados y fecha real, atrasado, vinculación N:M, otro proyecto, efectivos directos y heredados sin duplicar, avance y horas propias, mover un work item cambia sus heredados, fase de otro proyecto y borrado/cancelación.

- [ ] **T12 — Objetivos y resultados clave** (`internal/objectives`)
  - Depende de: T11, T9
  - RF: RF-97 a RF-115, RF-119
  - Hecho cuando: tests de alcances, activación sin resultados clave, meta igual a inicial, avance manual y derivado (work items y hitos), meta menor que inicial, vinculación automática del proyecto, hito no ligado al objetivo, desligar en cascada, hito cancelado, `milestone.completed`/`reopened`.

- [ ] **T13 — Plantillas** (`internal/projects`)
  - Depende de: T6, T7, T10 (para copiar contenido de ejemplo)
  - RF: RF-34 a RF-39
  - Hecho cuando: tests de plantilla oculta en listados y sin eventos; copia de configuración; copia con contenido (jerarquía, dependencias internas, hitos, estado inicial, sin asignado, fechas desplazadas); cambios en la plantilla que no tocan proyectos ya creados.

- [ ] **T14 — Matriz de permisos y auditoría**
  - Depende de: T13, T10, T12
  - RF: RF-124, RF-125, RF-127, RF-134, RF-135
  - Hecho cuando: `apitest/matrix_test.go` recorre todas las operaciones nuevas con cada rol y ámbito y obtiene el código de la matriz; un test comprueba que cada mutación deja su entrada de auditoría y que la descripción solo aparece como `description_changed`.

- [ ] **T15 — Prueba de rendimiento**
  - Depende de: T14
  - RF: requisitos no funcionales de tamaño y rendimiento
  - Hecho cuando: con el tamaño de diseño, cada umbral se cumple en local (mediana de 5); si no, se añade la tabla de cierre y se repite.

- [ ] **T16 — Frontend: base y catálogo** (`api/work.ts`, tipos, TipTap, componentes shadcn, `/admin/catalog`)
  - Depende de: T4, T5
  - RF: RF-136 a RF-138 (catálogo), RF-70
  - Hecho cuando: build y tests en verde (lista blanca compartida, i18n); desde el navegador se gestionan metodologías, fases, tipos, estados, prioridades, transiciones y campos.

- [ ] **T17 — Frontend: proyectos y plantillas** (`/projects`, `/projects/new`, `/projects/:id` resumen y fases)
  - Depende de: T16, T13
  - RF: RF-136, RF-137
  - Hecho cuando: build y tests en verde; desde el navegador se crea un proyecto desde plantilla, se filtra el listado, se editan datos, fases y gates.

- [ ] **T18 — Frontend: hitos y work items** (pestañas de hitos y work items, `/milestones/:id`, `/work-items/:key`)
  - Depende de: T17, T11, T10
  - RF: RF-136, RF-137
  - Hecho cuando: build y tests en verde (armado del árbol); desde el navegador se crea una épica con subactividades, se liga a un hito y se ven heredadas, se cambia el estado por el flujo, se crea una dependencia y se ve el conflicto de versión.

- [ ] **T19 — Frontend: objetivos** (`/objectives`, `/objectives/:id`)
  - Depende de: T18, T12
  - RF: RF-136, RF-137
  - Hecho cuando: build y tests en verde; desde el navegador se crea un objetivo con resultados clave, se ligan hitos y work items de dos proyectos y se ve el avance.

- [ ] **T20 — Documentación**
  - Depende de: T19
  - RF: —
  - Hecho cuando: `AGENTS.md` refleja los paquetes nuevos y la spec 004; ADR 0002 (eventos de dominio con outbox) y ADR 0003 (descripciones como documento JSON con lista blanca); prueba de humo en navegador de la demo.

- [ ] **T21 — Demo manual (tú, con apoyo)**
  - Depende de: T20
  - RF: todos (verificación)
  - Hecho cuando: se completan los pasos 1 a 14 de los criterios de finalización de la spec.
