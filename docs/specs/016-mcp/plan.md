# Plan — Spec 016: conexión de clientes de IA (MCP)

## Enfoque
- **Herramientas sobre la API existente.** Cada herramienta MCP se traduce en una o varias peticiones a las rutas de `/api` que ya usa la interfaz. Esas peticiones se sirven dentro del proceso, con el router de la app y con el principal de la persona dueña de la conexión en el contexto. Así se reutilizan tal cual:
  - permisos: el mismo motivo de rechazo que la interfaz (RF-27, RF-28);
  - validación;
  - auditoría y eventos (`events.Emit`, que alimenta el tiempo real y la gamificación: RF-45, RF-46);
  - conflictos de versión (RF-40).
- **Protocolo a mano, sin dependencias nuevas.** MCP usa el transporte *Streamable HTTP* en su forma mínima:
  - un `POST` con un mensaje JSON-RPC y respuesta `application/json`;
  - sin SSE ni sesiones.
  - Métodos: `initialize`, `notifications/*` (respuesta 202), `ping`, `tools/list` y `tools/call`.
  - Versiones aceptadas: `2025-11-25`, `2025-06-18`, `2025-03-26` y `2024-11-05`. Se devuelve la que pide el cliente si está en la lista y, si no, la más reciente.
- **Markdown ↔ documento TipTap, propio.** Un conversor pequeño en `internal/richtext` en las dos direcciones; todo lo que escribe pasa después por `Clean`, `CleanPage` o `CleanComment` (RF-34 a RF-36). Cubre:
  - bloques: párrafos, títulos 1–3, listas (anidadas, numeradas y de tareas en páginas), código, citas y separador;
  - marcas: negrita, cursiva, tachado, código y enlaces.

## URL de conexión
- La URL es `PUBLIC_URL + /api/mcp/<secreto>`. Pasa por el mismo proxy que el resto de `/api`, así que no hay que tocar Vite ni el despliegue.
- El secreto son 32 bytes aleatorios en base64url (256 bits, RF-10). Se guarda solo su `sha256` (RF-13).
- La ruta `/api/mcp/*` queda fuera del middleware CSRF: la llama un cliente sin `Origin` de Kairos.
- El log de peticiones escribe `/api/mcp/***` en vez del secreto (RNF de seguridad).
- Respuestas de error del punto MCP, siempre con un error JSON-RPC en el cuerpo y un mensaje en español:
  - secreto desconocido, caducado o revocado: `404` y el mismo mensaje en los tres casos (RF-22);
  - acceso por MCP desactivado: `403` (RF-3);
  - más de 120 peticiones por minuto por conexión: `429` con `Retry-After` (RF-47).
- Antes de cada petición se carga el principal con `PrincipalLoader` y se rechaza a la persona inactiva (RF-26).
- El último uso y el cliente que informa `initialize` (`clientInfo.name`) se anotan como mucho una vez por minuto.

## Despacho interno
- `auth.Internal(ctx)` marca el contexto de una petición interna. Se ponen en ese contexto el principal y la vía.
- `RequireSession` acepta el principal que ya viene en un contexto interno, y `CSRF` deja pasar la petición. Ninguna petición externa puede poner ese valor en el contexto.
- La respuesta interna se recoge con `httptest.ResponseRecorder`.
- Si responde `2xx`, el JSON se convierte en el resultado de la herramienta:
  - los campos que son documentos TipTap pasan a Markdown, con las etiquetas de `refs` para las menciones;
  - se añade `url`, el enlace a la pantalla de Kairos (RF-38);
  - las listas se cortan a 50 elementos (RF-39).
- Si no, el resultado lleva `isError: true` con el código, el mensaje y los detalles (RF-41).
- Las herramientas de edición leen el elemento, mezclan los campos recibidos y envían el `PATCH` con la versión leída. Si alguien cambia el elemento entre medias, devuelven `version_conflict` (RF-40).
- La descripción, que es la página del elemento, se edita con `PUT …/page` y el `page_version` leído. Hay dos modos: añadir al final, que es el modo por defecto, o reemplazar.

## Vía IA
- `audit.WithVia(ctx, Via{ConnectionID, Name, Client})`: `audit.Record` lee la vía del contexto y la guarda en `audit_log.via` (jsonb, RF-43).
- El historial de la tarea (spec 007) devuelve `via` en cada entrada (RF-44).
- Los comentarios guardan la vía en `work_item_comments.via` y la devuelven al leer (RF-44).

## Herramientas
**Lectura** (`readOnlyHint: true`):
- `my_dashboard`
- `search`: búsqueda global de proyectos, hitos y tareas por texto (RF-37)
- `list_projects`
- `get_project`
- `project_overview`
- `project_schedule`
- `portfolio`
- `tracking`
- `list_milestones`
- `get_milestone`
- `list_work_items`
- `get_work_item`: acepta el id o la clave legible; trae subtareas, dependencias y comentarios
- `my_points`
- `project_catalog`: tipos, estados, prioridades, tallas, etiquetas y miembros del proyecto

**Modificación** (`readOnlyHint: false`, `destructiveHint: false`):
- `create_work_item`
- `update_work_item`: campos, estado, asignados, talla, etiquetas y fechas
- `add_dependency`
- `remove_dependency`
- `add_comment`
- `create_milestone`
- `update_milestone`
- `update_project`
- `update_page`

No se ofrece ninguna herramienta que borre, archive, cancele o cierre algo, ni herramientas de administración (RF-31, RF-32).

## Datos (migración `00014_mcp.sql`, solo aditiva)
- `system_settings.mcp_enabled boolean NOT NULL DEFAULT false` (RF-1).
- Tabla `mcp_connections`:
  - `id`, `user_id`, `name`;
  - `secret_hash bytea UNIQUE`;
  - `created_at`, `expires_at`;
  - `last_used_at`, `last_client`;
  - `revoked_at`, `revoked_by`.
- `audit_log.via jsonb`.
- `work_item_comments.via jsonb`.

## API para la interfaz
- `GET /api/me/mcp-connections` devuelve `{ enabled, url_base, items }`.
- `POST /api/me/mcp-connections` recibe `{ name, days: 30|90|365 }` y devuelve la conexión con su `url`, una sola vez.
- `DELETE /api/me/mcp-connections/{id}` revoca una conexión propia.
- `GET /api/system/mcp-connections` y `DELETE /api/system/mcp-connections/{id}` requieren `system:manage` (RF-23, RF-24).
- `PUT /api/system/settings/mcp` recibe `{ enabled }`, con `system:manage`, y queda auditado (RF-7).
- Auditoría: `mcp.connection_created`, `mcp.connection_revoked` y `system.settings_changed` (RF-25).

## Frontend
- **Mi cuenta:** tarjeta «Conexiones de IA» con:
  - la lista de conexiones;
  - el alta con nombre y caducidad;
  - la URL mostrada una vez, con botón de copiar y el aviso de no compartirla;
  - la revocación;
  - los pasos generales (RF-8 a RF-19).
- **Sistema:** pestaña «IA (MCP)» con el interruptor y las conexiones de todos.
- **Historial y comentarios de la tarea:** la etiqueta «vía IA · nombre».
- Todos los textos en `es.json`.

## Tests
- **`richtext`:** Markdown → documento → `Clean`, y la vuelta.
- **apitest `mcp_test.go`:**
  - interruptor;
  - crear, listar y revocar conexiones;
  - caducidad;
  - persona desactivada;
  - límite de peticiones, con límite inyectable;
  - `initialize` y `tools/list`, sin herramientas destructivas;
  - consultas;
  - crear y editar tareas, y comentar, con «vía IA» en el historial y en el comentario;
  - rechazo por permisos con un observador;
  - conflicto de versión.
- **Matriz:** filas de `/api/system/mcp-connections` y `/api/system/settings/mcp`.
- **Vitest:** Mi cuenta (conexiones) y la pestaña de Sistema.
