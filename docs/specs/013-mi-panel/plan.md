# Plan 013 — Mi panel

Plan de implementación de [spec.md](spec.md) (aprobada 2026-11-04). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias y esquema
No hay dependencias nuevas ni migración:
- las tareas salen de las tablas de las specs 004 y 006;
- los proyectos y los papeles salen de las tablas de la spec 008;
- las cifras del plan salen de la spec 009.

Las gráficas se dibujan con SVG y HTML propios, sin librería.

## Backend (`internal/dashboard`)
- **Endpoint:** `GET /api/me/dashboard?today=`, con `work:read`.
  - Solo usa el usuario de la sesión, así que no hay forma de pedir el panel de otro (RF-3).
- **Consulta sqlc `DashboardTasks(user_id, since)`:**
  - devuelve las tareas asignadas al usuario en proyectos activos que no son plantilla, abiertas o cerradas desde `since`, que es el lunes de hace 11 semanas;
  - para cada tarea, el proyecto (con equipo y responsable para los permisos), el estado y su categoría, las fechas, `closed_at` y el peso.
- **Consulta sqlc `DashboardProjects(user_id)`:** los proyectos activos que no son plantilla en los que el usuario es responsable, es miembro del equipo o tiene tareas asignadas.
- **Cálculo en Go:**
  - **Kanban:** todas las abiertas y las cerradas en los últimos 14 días, cada una con `late_days` y `can_move` (`CanWriteWorkItems`). El orden y el tope de 100 se aplican en el cliente, por columna.
  - **Perfil:** abiertas, atrasadas y cerradas en el mes en curso (RF-6).
  - **Proyectos:**
    - el papel es «responsable» si el usuario lo es; si no, su papel en `ProjectRoleOf`; y si no tiene ninguno, «con tareas»;
    - cuentan sus tareas abiertas y atrasadas;
    - las cifras del proyecto salen de `portfolio.Load` con `Node.Own`;
    - el orden es el de RF-23.
  - **Series de 12 semanas,** de lunes a domingo, con la actual incluida: por semana, las tareas cerradas, la suma de sus pesos, las cerradas con fecha fin y las cerradas a tiempo (RF-28 a RF-32, RF-36).
  - **Carga:** las tareas abiertas y atrasadas por proyecto. Los 8 primeros van por su nombre y el resto se suma en «Otros» (RF-33, RF-34).
- **Tests:**
  - apitest del aislamiento entre usuarios, de qué tareas entran (proyectos cerrados fuera, cerradas hace más de 14 días fuera del Kanban pero dentro de las series), de los papeles, de la puntualidad y de «Otros»;
  - fila «ver mi panel» en la matriz.

## Frontend
- **Rutas:**
  - `/` pasa a `pages/HomePage.vue`. Con `work:read` muestra Mi panel; sin él, redirige a `/status`.
  - El estado del sistema queda en `/status`, con su entrada en la navegación (RF-1, RF-2).
- **API:** `api/dashboard.ts`.
- **Componentes en `features/dashboard/`:**
  - `ProfileCard.vue`: nombre, roles y equipos salen de `/api/me` (el store `auth`), y las tres cifras del panel (RF-5 a RF-7).
  - `MyKanban.vue` (RF-8 a RF-22):
    - columnas del catálogo de estados, más las retiradas en uso;
    - orden por atraso;
    - arrastre nativo, con los estados permitidos de `getWorkItem` al empezar a arrastrar, como el kanban del proyecto;
    - menú «Mover a» como alternativa;
    - guarda con `patchWorkItem`; si falla, muestra el error y vuelve a leer el panel.
  - `MyProjects.vue` (RF-23 a RF-27).
  - `ProgressCharts.vue`, con las tres gráficas: SVG o barras HTML con `role="img"` y texto alternativo, y cada una con su tabla de datos en un `<details>` (RF-28 a RF-35).
- **Textos:** `dashboard.*` en `es.json`.
- **Vitest:** perfil, columnas y orden, arrastre y su alternativa con flujo y error, proyectos, gráficas y tablas, y la redirección sin `work:read`.
