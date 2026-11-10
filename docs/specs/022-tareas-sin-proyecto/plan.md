# Plan — Spec 022: tareas sin proyecto

## Enfoque
Una tarea sin proyecto es un work item normal guardado en un **proyecto interno oculto** (la «bandeja»), único en toda la instalación. Así reutiliza estados, flujo, comentarios, páginas, historial, menciones, eventos y puntos sin tocar la regla de que todo work item pertenece a un proyecto, y la migración es solo aditiva.

- La bandeja nunca es visible como proyecto: `projects.Visible` devuelve falso para ella, de modo que todo endpoint de proyecto (ficha, árbol, kanban, hitos, informes, vistas, reuniones, objetivos) responde 404, y los listados (Proyectos, Portafolio, Seguimiento, tablero de proyectos, Mi panel, menciones) la excluyen.
- Se crea sola con la primera tarea sin proyecto: responsable, el creador; equipo, el primero del creador o, si no tiene, el más antiguo no archivado; tipos, todos los activos; identificador `TAREA` (o `TAREA2`, `TAREA3`… si ya existe).
- El acceso a sus work items no sale del proyecto sino de la tarea (RF-14 a RF-17): `workitems.Load` y las comprobaciones de escritura y borrado pasan por `loose.Access`.

## Datos (migración 00019, aditiva)
- `projects.is_inbox boolean NOT NULL DEFAULT false` con índice único parcial (una sola bandeja).
- `work_items.team_id uuid NULL REFERENCES teams` (equipo destino) y `work_items.team_mode text NULL CHECK (team_mode IN ('claim','each'))`.
- «Cada persona la hace»: la tarea del equipo es un work item sin asignados (`team_mode='each'`, sin padre) y la de cada miembro es su hija con el mismo `team_id` y modo. El avance del equipo sale de las hijas cerradas.

## Backend
- `internal/authz`: `loose:assign:all` (admin), `loose:assign:team` (líder), `loose:create:own` (admin, líder, miembro), en la matriz y en `matrix_work_test.go`.
- `internal/loose` (nuevo): bandeja, permisos de acceso, alta (persona, equipo «toma», equipo «cada persona» con reparto), tomar y devolver (con bloqueo de fila: RF-20), «Para tomar», tareas del equipo para Mi equipo, alta de quien entra en un equipo (RF-25, llamado desde `users` y desde el login OIDC) y paso a un proyecto (RF-37 a RF-39).
- `internal/workitems`: `Load`, `Update`, `Delete` y `view` consultan `loose` para la bandeja; sin padre ni dependencias en la bandeja; cerrar la última hija cierra el grupo y reabrir una lo reabre (RF-27); borrar el grupo borra las hijas abiertas (RF-28); la vista lleva `loose` (equipo, modo, avance, `can.claim`, `can.release`, `can.move`).
- `internal/dashboard`: tareas sin proyecto en el Kanban con su marca; fuera de proyectos, cifras y carga (RF-29 a RF-32).
- `internal/workload`: las barras de la bandeja salen marcadas como sin proyecto (RF-36).
- Consultas de listados con `NOT p.is_inbox`: lista de proyectos, portafolio, seguimiento, tablero de proyectos, Mi panel (proyectos), menciones.

## Frontend
- `api/loose.ts`; diálogo «Nueva tarea» (diseño, pantalla 1) en Mi panel y Mi equipo.
- Mi panel: sección «Para tomar en tu equipo», marca en las tarjetas y filtro por origen.
- Mi equipo: pestañas «Personas» y «Tareas del equipo».
- Ficha de tarea: «Sin proyecto» o el equipo, «La tomo», «Devolver al equipo» y «Mover a un proyecto».
- Cronograma por persona: color «Sin proyecto» en la leyenda.

## Tests
- API: matriz de permisos, alta en cada modo, toma simultánea, devolver, reparto y alta en el equipo, cierre del grupo, borrado del grupo, paso a un proyecto, puntos al cerrar, que la bandeja no salga en Proyectos, Portafolio ni Seguimiento y que su ficha de proyecto dé 404.
- Frontend: diálogo, «Para tomar», filtro de Mi panel, pestaña de Mi equipo, acciones de la ficha.
