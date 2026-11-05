# Plan — Spec 017: notificaciones

## Enfoque
- **Sucesos como eventos de dominio.** Cada suceso se registra con `events.Emit`, en la misma transacción que el cambio (ADR 0002).
- **Consumidor nuevo.** Un consumidor `notifications`, registrado en el `Dispatcher` que ya existe (revisa la cola cada segundo), calcula los destinatarios y crea o agrupa las notificaciones. Guardar un comentario no espera a las notificaciones (RNF de rendimiento).
- **Tiempo real con el WebSocket que ya existe** (`/ws`, `realtime.Hub`). El hub gana un `Send(userID, mensaje)`. Tras guardar, el consumidor envía la notificación a todas las conexiones de cada destinatario (RF-33, RF-34). Al marcar como leídas, el servidor envía el contador nuevo a las demás pestañas (RF-35).
- **Sin componentes nuevos.** No hay broker, colas externas ni servicio de push. La migración arranca el cursor del consumidor en el último evento, así que no se avisa de sucesos anteriores (fuera de alcance).
- **Visibilidad.** Cualquiera con `work:read` ve todos los proyectos que no son plantilla, y las plantillas no generan sucesos. Por tanto, «puede ver el elemento» equivale a estar activo y tener `work:read` (RF-16). La lista oculta las notificaciones cuyo elemento ya no existe (RF-45).

## Eventos nuevos (`internal/events`)
Todos los sucesos tienen como sujeto el elemento (`work_item`, `milestone` o `project`). Ninguno se emite en plantillas.

| Tipo | Dónde se emite | Payload |
|---|---|---|
| `comment.created` | `comments.Create` | `comment_id`, `thread_id` (la raíz), `reply`, `mentions` (ids de usuario), `excerpt` (140 caracteres) |
| `comment.thread_resolved` / `comment.thread_reopened` | `comments.SetResolved` | `thread_id` |
| `work_item.status_changed` | `workitems.Update` si cambia `status_id` | `from_status_id`, `to_status_id` |
| `mention.added` | `events.EmitMentions` con la descripción de antes y la de después, en los 7 sitios donde se guarda una descripción: `pages.Save`, más `Create` y `Update` de tareas, hitos y proyectos | `user_ids`: solo las menciones nuevas de tipo `user` (RF-11) |

## Seguir tareas
- Tabla `work_item_follows(work_item_id, user_id, following, updated_at)`. `following = false` significa que la persona dejó de seguir la tarea a propósito.
- Un trigger `AFTER INSERT` en `work_item_assignees` la marca como seguidora `ON CONFLICT DO NOTHING`. Así cubre todos los caminos, también las copias, sin pisar un «dejar de seguir» cuando se reescriben los asignados (RF-1).
- `workitems.Create` y `workitems.Update` vuelven a marcar como seguidores a los asignados recién añadidos, aunque hubieran dejado de seguir la tarea (RF-5).
- Quitar un asignado no cambia nada (RF-6).
- La migración marca como seguidores a los asignados actuales.
- API: `GET /api/work-items/{id}/follow` y `PUT /api/work-items/{id}/follow { following }`, con `work:read` y la tarea visible (RF-2, RF-3).

## Notificaciones
- Tabla `notifications`:
  - `id`, `user_id`, `subject_type`, `subject_id`, `project_id`;
  - `actors uuid[]`: el más reciente primero, sin repetidos;
  - `events`, `mention`, `last_kind`, `last_data jsonb`, `via jsonb`;
  - `read_at`, `created_at`, `updated_at` (hora del último suceso).
- Índice único parcial `(user_id, subject_type, subject_id) WHERE read_at IS NULL`. El `INSERT … ON CONFLICT … DO UPDATE` agrupa los sucesos mientras la notificación no se lea (RF-21, RF-22, RF-25).
- `notified_events(event_id, user_id)`: la entrega es «al menos una vez», y esta tabla evita contar dos veces el mismo suceso.
- Destinatarios por suceso:
  - quitando siempre al autor (RF-15) y a quien no esté activo o no tenga `work:read` (RF-16);
  - con un solo aviso por persona, que cuenta como mención si alguna de sus razones lo es (RF-17).

  | Suceso | Destinatarios |
  |---|---|
  | comentario raíz | seguidores |
  | respuesta | seguidores, más los participantes del hilo que no hayan dejado de seguir la tarea |
  | mención | la persona mencionada, la siga o no |
  | estado | seguidores |
  | hilo resuelto o reabierto | participantes que no hayan dejado de seguir la tarea |

- Lectura: una consulta sqlc une cada notificación con su elemento (clave, título o nombre), los nombres de los dos primeros autores y los nombres de los estados. El extracto del comentario desaparece si se borró.
- API:
  - `GET /api/notifications?unread=&page=&page_size=` devuelve `{ items, total, unread, page, page_size }`, con las no leídas primero (RF-27, RF-28, RF-29);
  - `GET /api/notifications/unread` devuelve el contador;
  - `PUT /api/notifications/{id}/read { read }`;
  - `POST /api/notifications/read-all`.
- Al marcar como no leída una notificación de un elemento que ya tiene otra sin leer, esta se queda como estaba: el elemento ya figura como no leído.
- `notifications.Run`: cada hora borra lo que lleva más de 90 días sin sucesos (RF-44).

## Frontend
- `composables/useRealtime.ts`: una sola conexión `/ws` por pestaña.
  - Reconecta con espera creciente, de 1 s a 30 s (RF-36).
  - Al reconectar, vuelve a leer las notificaciones (RF-37).
  - Mientras tanto, el contador se consulta cada 60 s (RF-38).
- `features/notifications/`:
  - `NotificationBell`: campana en la barra lateral con contador «99+» y desplegable de 20;
  - `NotificationItem`;
  - `NotificationToasts`: avisos de 6 s, agrupados si llegan más de 3; no salen si ya está abierta esa ficha; con `role=status`;
  - `FollowButton`, en la ficha de la tarea.
- `pages/NotificationsPage.vue` en `/notifications`, con el filtro de todas o no leídas.
- Pulsar una notificación la marca como leída y abre el elemento. Si el último suceso es un comentario, lleva a ese comentario (RF-39 a RF-41).
- Todos los textos en `es.json`.

## Tests
- **apitest `notifications_test.go`:**
  - destinatarios de cada suceso;
  - el autor excluido;
  - seguir y dejar de seguir;
  - la mención que llega aunque no se siga la tarea;
  - agrupación y que al leer empiece una notificación nueva;
  - marcar como leída, como no leída y todas;
  - el elemento borrado;
  - el tiempo real, con un cliente WebSocket de prueba;
  - la entrega repetida del mismo evento.
- **Matriz:** fila de seguir tarea.
- **Vitest:** campana, lista, página, avisos emergentes, Seguir y reconexión.
