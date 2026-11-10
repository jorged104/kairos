# Plan — Spec 023: quién ve cada proyecto, y el rol Dirección

## Enfoque
La visibilidad se calcula **una vez por usuario**, al cargar su `authz.Principal` (con la caché de 60 s que ya existe; RF-4), y la consultan todas las piezas por una sola puerta:

- `Principal.SeesProject(ref)`: ve todos (`projects:read:all`), o es responsable, o tiene papel, o el equipo del proyecto está en sus equipos vistos (los suyos, sus antecesores y sus descendientes), o en su ámbito de líder (`projects:read:team`), o tiene tareas asignadas en él, o es un programa con algún descendiente visible.
- `Principal.VisibleProjectIDs()`: el conjunto ya calculado, para filtrar en SQL los listados paginados (`p.id = ANY(...) OR p.owner_id = yo`, para que un proyecto recién creado se vea sin esperar a la caché).
- `Principal.ReadsByAssignment(ref)`: lo ve solo por tener tareas (RF-15).
- `projects.Visible` (ya usado por los 13 paquetes que leen un proyecto con `projects.Get`) pasa a consultar `SeesProject`, así que fichas, hitos, tareas, páginas, comentarios, archivos, reuniones e informes quedan cubiertos sin tocarlos (RF-3).

## Datos (migración 00020, aditiva)
- `user_roles.role` admite `director` (se amplía el CHECK).
- Consultas nuevas: equipos directos, equipos vistos (antecesores y descendientes), proyectos con tareas asignadas y la lista mínima de proyectos (id, padre, equipo, responsable, programa) para calcular el conjunto.

## Backend
- `internal/authz`: rol `director` (entre `admin` y `lead`), permisos `projects:read:all`, `projects:read:team`, `oversight:view` (Portafolio y Seguimiento) y `workflow:override`, en la matriz y en sus tests. `InTeamScope` protege también a Dirección (RF-10).
- `internal/auth/principal.go`: carga los equipos directos y vistos y los proyectos asignados, y calcula el conjunto visible.
- Listados filtrados: `/projects`, `/projects/{id}/subtree`, `/project-board`, `/portfolio` (programas y proyectos visibles, cifras con todos, `hidden` por programa), `/tracking` (acceso con `oversight:view`, «mis proyectos» de RF-21 y valores de filtro visibles), `/mentions`, `/objectives` (proyectos ligados visibles más `hidden_projects`), Mi panel de otra persona y cronogramas (`hidden: true`, sin título, clave ni proyecto; RF-26), informes con IA (subproyectos visibles).
- `/projects/{id}/members` para quien ve el proyecto; el cronograma del equipo sigue para quien lo gestiona (RF-12, RF-13).
- `workitems`: quien solo lee por tarea asignada cambia estado, avance y horas de sus tareas (RF-15).
- `catalog.AllowedTargets`: `workflow:override` en lugar de `HasRole(admin)`; Dirección usa las transiciones de `member` (RF-30).
- MCP: las herramientas `portfolio` y `tracking` solo aparecen con `oversight:view` (RF-17a).

## Frontend
- Rol «Dirección» en tipos, textos y alta de roles.
- Portafolio y Seguimiento en la navegación y en las rutas solo con `oversight:view`; sin enlaces a ellos para quien no lo tiene (RF-17c).
- Portafolio: «N proyectos que no ves» por programa; objetivos, lo mismo.
- Panel y cronograma de otra persona: «Tarea de otro proyecto», sin enlace.

## Tests
- API: tabla de visibilidad (una persona por condición más otra sin acceso) contra ficha, listados, tablero, portafolio, seguimiento, menciones, objetivos y MCP; Dirección en la matriz; acceso de lectura por tarea (RF-15, RF-16); Portafolio con programa mixto.
- Frontend: menú sin Portafolio ni Seguimiento para miembros, aviso de proyectos ocultos y tarea oculta en el panel.
