# Plan — Spec 018: reuniones

## Enfoque
La reunión es un dueño de página más, como la tarea, el hito y el proyecto. Reutiliza las piezas que ya existen en lugar de copiarlas:
- **Páginas (spec 005).** `pages` gana el tipo `meeting`, con el mismo guardado automático (`PUT /meetings/{id}/page`) y la misma subida de archivos. `page_files.owner_kind` admite `meeting` y `meeting_comment`.
- **Comentarios (spec 006).** `work_item_comments` gana `meeting_id`, y `work_item_id` pasa a ser opcional: cada comentario lleva exactamente uno de los dos. El servicio de comentarios trabaja sobre un *destino* (tarea o reunión) que aporta el proyecto, la página y su versión. Las rutas `/comments/{id}…` no cambian, y se añaden `/meetings/{id}/comments` y `/meetings/{id}/comment-files`.
- **Menciones.** La lista blanca admite menciones de tipo `meeting`. Tanto la búsqueda `#` como la resolución de etiquetas incluyen reuniones.
- **Notificaciones (spec 017).**
  - Las menciones en las notas o en los comentarios, y las respuestas en sus hilos, emiten los mismos eventos con sujeto `meeting`.
  - `notifications.subject_type` admite `meeting`. Como las reuniones no tienen seguidores, se avisa solo a los mencionados y a los participantes del hilo.
- **Tareas.** `work_items.meeting_id`, con `ON DELETE SET NULL` (RF-13, RF-26). Se crea la tarea con `POST /projects/{id}/work-items` y `meeting_id`, que debe ser una reunión del mismo proyecto. La ficha de la tarea devuelve `meeting {id, title}` (RF-28).
- **Ampliable.** La reunión guarda `owner_type` (hoy solo `project`) y `owner_id`. Los permisos y el proyecto de referencia se resuelven según `owner_type`, así que otro dueño en el futuro es un caso más del `switch`.

## Datos (migración `00016_reuniones.sql`, solo aditiva)
- **`meetings`:**
  - `id`, `owner_type`, `owner_id`, `project_id`: el proyecto de referencia, para los permisos y los archivos;
  - `title`, `starts_at timestamptz`, `location`;
  - `description`, `description_text`, `page_version`, `version`;
  - `template_id`, `created_by`, `created_at`, `updated_at`.
  - Índice `(project_id, starts_at)`.
- **`meeting_templates`:**
  - `id`, `project_id`: nulo en las globales;
  - `name`, `name_normalized`, `title`, `location`, `page`, `position`;
  - `retired_at`, `created_at`, `updated_at`.
  - Índice único por ámbito (`coalesce(project_id, cero)`, `name_normalized`).
- **`work_items.meeting_id`.**
- **`work_item_comments`:** gana `meeting_id`, `work_item_id` deja de ser obligatorio y se añade un CHECK de exactamente uno.
- **Restricciones ampliadas:**
  - `page_files.owner_kind` admite `meeting` y `meeting_comment`;
  - `notifications.subject_type` admite `meeting`.

## Permisos (sin permisos nuevos)
| Acción | Quién |
|---|---|
| Leer | `work:read`; el proyecto debe ser visible |
| Crear, editar y borrar reuniones, y crear tareas desde ellas | `CanWriteWorkItems` del proyecto, con el proyecto activo |
| Comentar | `CanComment`, con el proyecto activo |
| Plantillas globales | `catalog:manage` |
| Plantillas del proyecto | `CanManageProject` |

## API
- **Reuniones:**
  - `GET /projects/{id}/meetings?when=upcoming|past&page=&page_size=` (máximo 50);
  - `POST /projects/{id}/meetings { title, starts_at, location, template_id?, description? }`;
  - `GET`, `PATCH { version, title, starts_at, location }` y `DELETE` sobre `/meetings/{id}`;
  - `PUT /meetings/{id}/page` y `POST /meetings/{id}/files`, por `pages`.
- **Plantillas:**
  - `GET /meeting-templates?project_id=`: las globales activas y las del proyecto, para elegir;
  - `GET /meeting-templates/global`: todas, para el catálogo;
  - `POST /meeting-templates { project_id?, name, title, location, page }`;
  - `PATCH /meeting-templates/{id} { name, title, location, page, position }`;
  - `POST /meeting-templates/{id}/retire|restore`, solo las globales;
  - `DELETE /meeting-templates/{id}`, solo las del proyecto.
- **Auditoría:**
  - `meeting.created`, `meeting.updated`, `meeting.deleted` y `meeting.page_updated`, esta última sin contenido;
  - `meeting_template.*`.

## MCP (spec 016)
- **Consulta:** `list_meetings`, `get_meeting` y `list_meeting_templates`.
- **Modificación:** `create_meeting`, `update_meeting`, `add_meeting_comment` y `create_meeting_task`.
- `update_page` admite `kind = meeting`.
- Una fecha sin zona horaria se rechaza (RF-52).

## Frontend
- **Proyecto:** pestaña «Reuniones» (`MeetingsTab`):
  - próximas y pasadas;
  - «Nueva reunión» con plantilla;
  - plantillas del proyecto, para quien lo gestiona.
- **Ficha de la reunión:** `MeetingSheet`, con campos, notas (`PageSection`), tareas y comentarios (`CommentsSection`, generalizado al destino).
  - En el panel lateral (`?meeting=`), integrado en `ItemPanel`.
  - En su propia página `/meetings/:id`.
- **«Crear tarea»** junto a la selección de las notas: elegir tipo y título, crear la tarea con `meeting_id` e insertar la mención tras la selección.
- **Catálogo:** sección «Plantillas de reunión» para las globales.
- **Portafolio:** las 3 próximas reuniones de cada programa.
- **Otros sitios:** menciones `#` con reuniones, notificaciones de reuniones y la ficha de la tarea con su reunión.

## Tests
- **apitest `meetings_test.go`:**
  - crear, listar, editar y borrar, con permisos y proyecto cerrado;
  - plantillas, por ámbito, duplicados, retirar y la copia al crear;
  - tareas desde la reunión;
  - comentarios y notificaciones;
  - menciones;
  - borrar la reunión conservando sus tareas.
- **apitest de las herramientas MCP de reuniones.**
- **Filas de la matriz.**
- **Vitest:** pestaña, ficha, «Crear tarea», plantillas y portafolio.
