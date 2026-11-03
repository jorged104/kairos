# Plan 011 — Seguimiento de proyectos

Plan de implementación de [spec.md](spec.md) (aprobada 2026-11-03). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias y esquema
Ninguna dependencia nueva y ninguna migración: todo sale de las tablas de las specs 004, 009 y 010.

## Backend (`internal/tracking`)
- **Endpoint:** `GET /api/tracking?program_id=&project_id=…&owner_id=…&today=`, con `work:read` (RF-1).
  - **Respuesta:** `{today, matched, projects, summary, columns}`.
  - Los valores de filtro que no existen se ignoran.
- **Selección de proyectos:**
  - La consulta sqlc `TrackingProjects` devuelve los proyectos activos que no son plantilla, con equipo y responsable.
  - El programa se resuelve con `portfolio.Tree.Members` (RF-2), y los filtros se combinan con «y» (RF-5).
  - Sin filtros se siguen los proyectos con `ProjectRoleOf != ""` y, si no hay ninguno, todos (RF-6, RF-7).
  - El resultado se ordena por nombre y se corta en 50, guardando `matched` (RF-9).
- **Indicadores:** el plan y el real de cada proyecto salen de `portfolio.Load`, que ahora también devuelve las tareas de cada proyecto. La media ponderada por `Node.Weight` se hace con `portfolio.Combine`, que es el mismo cálculo de `Aggregate` sacado a una función (RF-13).
- **Hitos:**
  - La consulta `TrackingMilestones(ids)` devuelve los hitos no cancelados, y `TrackingMilestonePairs(ids)` sus tareas efectivas.
  - **Barra:** del mínimo de los inicios planificados (o del fin, si no hay inicio) al máximo de los fines planificados (RF-16).
  - **Plan:** `schedule.Plan` sobre sus tareas.
  - **Real:** el peso de las cerradas entre el peso total, como `MilestoneProgress`. `schedule.Item` gana `Closed`.
  - **Estado:** las clasificaciones se aplican en orden (RF-20 a RF-26).
- **Tareas:** la consulta `TrackingLeafItems(ids)` devuelve las tareas sin hijas con clave, título, padre, fechas, estado (inicial o cerrado), `closed_at` y avance derivado. Se reparten en las cuatro columnas en Go, con su orden y un tope de 100 por columna (RF-30 a RF-41).
- **Tests:**
  - unitarios de la clasificación de hitos y del reparto en columnas;
  - apitest de filtros, valores por defecto, tope de 50 e indicadores;
  - fila «ver seguimiento» en la matriz.

## Frontend
- **API:** `api/tracking.ts`.
- **Página `pages/TrackingPage.vue`:**
  - ruta `/tracking` y entrada «Seguimiento» en la navegación;
  - filtros en la query de la ruta (RF-8): selector de programa (de `getPortfolio`), proyectos con búsqueda (de `listProjects`) y responsables (de `usePeople`).
- **Componentes en `features/tracking/`:**
  - `MilestoneGantt.vue`: eje de meses, línea de hoy, barra con el real, rombo en la fecha objetivo y leyenda;
  - `TaskColumns.vue`: las cuatro columnas con su contador y «N más».
- **Textos:** `tracking.*` en `es.json`.
- **Vitest:** filtros en la URL, indicadores, clasificaciones del Gantt, columnas y enlaces.
