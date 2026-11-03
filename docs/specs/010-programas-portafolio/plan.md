# Plan 010 — Programas y portafolio

Plan de implementación de [spec.md](spec.md) (aprobada 2026-11-03). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias nuevas
Ninguna.

## Esquema (migración `00010_programas.sql`, solo aditiva)
- `projects.is_program boolean NOT NULL DEFAULT false`.
- `projects.portfolio_weight int NULL CHECK (0..100000)`. `NULL` es el esfuerzo (RF-6).

## Autorización
- Permiso nuevo `portfolio:manage`, solo para el admin, con fila en la matriz de `authz` y en `apitest/matrix_work_test.go` (RF-3, RF-7).
- Ver el portafolio pide `work:read` (RF-21).
- `Project.can.manage_portfolio`.

## Backend
- **`PUT /api/projects/{id}/portfolio` con `{version, is_program, weight}`** (`projects/portfolio.go`):
  - exige `portfolio:manage`;
  - rechaza las plantillas (RF-4);
  - comprueba `version` y la sube;
  - audita `project.program_set` (RF-9) y `project.portfolio_weight_changed` con el antes y el después (RF-10);
  - devuelve el proyecto.
- **`Project` gana** `is_program`, `portfolio_weight` y `effort`, la Σ de los pesos de sus tareas propias, para RF-8.
- **Cálculo (`internal/portfolio`, funciones puras con tests de tabla):**
  - **Entrada:** `Node{ID, ParentID, Name, Identifier, Lifecycle, IsProgram, Archived, ManualWeight, Effort, Own Figures, OwnPlan/HasPlan, OwnReal}`.
  - **Peso:** `Weight(n)` es `ManualWeight` o, si no tiene, `Effort`.
  - **Agregado de un programa:** recorre el subárbol una sola vez (RF-13). Los nodos archivados no cuentan, pero se baja por ellos. Hace la media ponderada del real (RF-11) y del plan de quienes tienen plan (RF-12), con media simple si todos pesan 0 (RF-17), y calcula la brecha y el nivel con `schedule.Make` (RF-19). Un programa cerrado se queda sin plan (RF-20).
  - **Tarjetas:** una por programa no archivado (RF-22). Llevan `projects_count`, `below_plan` (nivel `at_risk` o `late`, RF-24) y las filas de los hijos directos: los programas con su agregado (RF-27) y el resto con lo suyo (RF-26). La tarjeta «Sin programa» recoge los proyectos sin un programa por encima (RF-28).
- **Consultas sqlc:**
  - `PortfolioProjects`: los proyectos que no son plantilla, archivados incluidos para poder recorrer el árbol, con su esfuerzo.
  - `PortfolioPlanItems`: las tareas de los proyectos no archivados, con el mismo `LEFT JOIN` a la línea base que `ProjectPlanItems`, más `project_id`.
- **Plan del proyecto:** el plan propio con el respaldo lineal pasa a `schedule.ProjectPlan(start, end, items, day)`, para que lo compartan el Resumen y el portafolio.
- **Endpoint del portafolio:** `GET /api/portfolio?today=` devuelve `{today, programs, loose}`.
- **Resumen de un programa:** `Overview` gana `program`, con su agregado, el recuento y las filas, y se omite si el proyecto no es programa (RF-39, RF-40).

## Frontend
- **API:** `api/portfolio.ts`, con `getPortfolio` y `setPortfolio`.
- **`pages/PortfolioPage.vue`:**
  - ruta `/portfolio` y entrada «Portafolio» en la navegación (RF-21);
  - tarjetas con su `LevelBadge`; las filas llevan barra con la marca del plan y enlazan al Resumen (RF-30, RF-31);
  - filtro por niveles y «Sin plazo», orden por columna con brecha ascendente por defecto, y las dos elecciones en `localStorage` (RF-32 a RF-38).
- **`features/portfolio/ProgramRows.vue`:** la tabla de filas, que se reutiliza en el Resumen del programa.
- **Datos del proyecto:** el administrador ve la casilla «Es un programa» y el peso. El campo vacío quita el peso fijado; si no hay peso fijado se muestra «Esfuerzo: N».
- **Textos:** todos en `es.json`, en el bloque `portfolio.*`.

## Tests
- **Unitarios Go:** el agregado contando cada nodo una vez con programas anidados, los archivados, el peso fijado frente al esfuerzo, la media simple con peso 0, «Sin plazo» y el programa cerrado.
- **apitest:**
  - marcar y desmarcar, permisos, plantilla, `version_conflict`, peso fuera de rango y auditoría;
  - el portafolio con programas anidados, la tarjeta «Sin programa» y un proyecto archivado;
  - el Resumen del programa.
- **Matriz:** «ver portafolio» para todos y «gestionar programa» solo para el admin.
- **Vitest:**
  - tarjetas y filas, filtro, orden y enlaces del portafolio;
  - controles del administrador en los datos del proyecto.
