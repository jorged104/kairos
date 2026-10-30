# Plan 006 — Tallas, varios asignados y comentarios

Plan de implementación de [spec.md](spec.md) (aprobada 2026-10-29). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Dependencias nuevas
Ninguna. El editor de comentarios y la marca de anclaje usan TipTap 3.31.4 y sus extensiones,
ya instaladas (spec 005). El botón «Comentar» sobre la selección es un elemento propio posicionado
con `coordsAtPos`, sin `extension-bubble-menu`.

## Esquema (migración `00006_tallas_asignados_comentarios.sql`, solo aditiva)
- **`work_item_sizes`:** `id`, `name`, `name_normalized`, `weight smallint CHECK 0..100`,
  `position`, `retired_at`, fechas; nombre único entre las activas. Se siembra con XS 1, S 2,
  M 3, L 5 y XL 8 (RF-2).
  - Cambio 2026-10-30 (RF-57 a RF-59): el código deja de leer y escribir `weight`. La columna
    se queda (con su valor por defecto) para la versión anterior de la API y se borrará en un
    despliegue posterior.
- **`work_items.size_id`:** nullable, FK a `work_item_sizes`.
- **`work_item_assignees`:** `(work_item_id, user_id)` como PK, `position`, `created_at`; índice
  por `user_id` para el filtro (RF-19). Se rellena desde `assignee_id` (RF-16).
- **Compatibilidad con la API anterior:** `work_items.assignee_id` se mantiene y la API nueva lo
  escribe con el primer asignado. Un trigger `AFTER INSERT OR UPDATE OF assignee_id` sustituye la
  lista por `[assignee_id]` (o la vacía) solo si `assignee_id` no está ya en ella. Así un cambio
  de la API anterior se refleja y uno de la nueva no se pisa: la nueva escribe la lista antes de
  la fila.
- **`work_item_comments`:** `id`, `work_item_id` (FK con `ON DELETE CASCADE`, RF-40),
  `thread_id` (null en el raíz, FK al raíz), `author_id`, `body jsonb`, `body_text`, `quote`
  (texto comentado, RF-48), `anchored bool`, `created_at`, `edited_at`, `deleted_at` (raíz
  borrado con respuestas, RF-33), `resolved_at` y `resolved_by` (solo en el raíz).
- **`page_files.owner_kind`:** el CHECK admite `work_item_comment`. Los archivos de los
  comentarios de un work item tienen como dueño `(work_item_comment, work_item_id)`.

## Autorización
- Permiso nuevo `work:comment` en la matriz para `admin`, `lead` y `member`, no para `viewer`
  (RF-23). `Principal.CanComment()`. La visibilidad del work item es la de su proyecto
  (`projects.Get`).
- Editar un comentario: solo su autor (RF-31, RF-35). Borrar: su autor o quien cumple
  `CanManageProject` (RF-34). Resolver y reabrir: quien puede comentar.
- Proyecto no activo: `projects.Writable` en toda escritura (RF-39).
- Test de matriz: crear, responder, editar, borrar, resolver y subir archivo, por rol.

## Backend
- **`catalog`:** tallas con el mismo patrón que las prioridades (`mutate`, auditoría,
  `retire`/`restore`): rutas `/work-item-sizes`; `Catalog.sizes`.
- **`workitems`:**
  - `size_id` en las peticiones. ~~Al cambiar de talla (o al crear con talla y sin peso), el peso
    pasa a ser el de la talla e ignora el que llega (RF-5); si no cambia, el peso es el que
    llega (RF-6).~~ La talla no toca el peso (RF-59). Una talla retirada solo se acepta si ya
    era la suya (RF-9).
  - `assignee_ids *[]uuid` en las peticiones. Si no llega, se usa `assignee_id` como lista de
    uno (clientes anteriores). Se validan los que se añaden (RF-14), sin duplicados, máximo 20
    (RF-13). Respuesta con `assignees: []Ref` y `assignee` (el primero) por compatibilidad.
  - Árbol y listado: `assignees` agregados con `json_agg` por fila; filtro por `EXISTS`.
  - Eventos con `assignee_ids` (RF-20); auditoría con `assignees_added` y
    `assignees_removed` (RF-21).
  - Copia desde plantilla: `size_id` (RF-11).
- **`richtext`:**
  - Marca `comment` con `threadId: uuid`, solo en páginas.
  - `CleanPage` devuelve los hilos anclados (`Page.Threads`).
  - `StripComments(doc)` quita la marca y une los textos contiguos con las mismas marcas.
  - `CleanComment(raw, files)` usa los nodos de página sin la marca y sin bloques de arrastre
    (los mismos nodos), con un límite de 10 000 caracteres (RF-27).
- **`comments` (paquete nuevo):** `GET /work-items/{id}/comments`,
  `POST /work-items/{id}/comments`, `PATCH /comments/{id}`, `DELETE /comments/{id}`,
  `POST /comments/{id}/resolve|reopen` y `POST /work-items/{id}/comment-files`. Todas con
  `audit.Record` en la transacción (RF-42).
  - **Anclar** (RF-45, RF-46): el `POST` lleva `anchor {thread_id, quote, page, page_version}`.
    El hilo usa ese id. En la misma transacción se compara `page_version` (si no,
    `page_conflict`) y se comprueba que `StripComments(clean(page)) == StripComments(stored)`
    (si no, `anchor_page_changed`). Después se guarda la página. Así quien no puede editar solo
    añade su marca.
  - **Lectura:** los hilos con sus respuestas en dos consultas. El texto «eliminado» se calcula
    con los `threadId` de la página guardada (RF-51). Las menciones se resuelven con una sola
    llamada a `mentions.Refs` sobre todos los cuerpos.
  - Responder a un hilo resuelto lo reabre (RF-38). Borrar la última respuesta de un raíz
    borrado quita el hilo.

## Frontend
- **Tipos y API:** `size_id`, `assignees`, `Catalog.sizes`; `api/comments.ts`.
- **Catálogo:** tabla de tallas (nombre, peso, orden, retirada) en la administración.
- **Ficha:** talla y peso entre las principales (RF-10); `AssigneesPicker` (lista con
  búsqueda y casillas, guarda al cerrar) en la ficha y en la celda del árbol (RF-17).
  Avatares con iniciales y «+N» (RF-18).
- **Comentarios** (`features/comments`):
  - `CommentsSection` va entre las propiedades y la página (RF-22).
  - `CommentEditor` usa `PageEditor` en variante compacta, sin asas ni menú `/` (las menciones
    sí), con subida a `comment-files`.
  - Hilos resueltos plegados (RF-36).
- **Anclajes:** marca `comment` en `pageExtensions` con `inclusive: false` y `excludes: ''`
  (solapes, RF-54); al pegar o duplicar se quita (RF-53).
  - **Botón «Comentar»:** flotante sobre la selección (RF-44).
  - **Al publicar:** se vacía el autoguardado, el editor pasa a solo lectura mientras se envía,
    y la página con la marca se calcula sobre el estado sin despacharla. Si sale bien, se aplica
    la marca sin historial ni autoguardado y se adopta la `page_version` nueva.
  - **Resaltado:** solo de los hilos abiertos (RF-49), con una hoja de estilo generada
    (`useStyleTag`).
  - **Al pulsar un resaltado:** se abre el hilo flotante junto al texto (RF-47, RF-54).

## Tests
- **apitest:**
  - **Tallas:** fijan el peso, peso manual, retiradas.
  - **Asignados:** lista, límite, inválidos, filtro, eventos y conversión.
  - **Compatibilidad:** un `PATCH` con solo `assignee_id`.
  - **Comentarios:** CRUD, permisos, moderación, resolver y reabrir, proyecto cerrado,
    archivos, menciones.
  - **Anclajes:** de quien no edita, rechazo si cambia otra cosa, texto eliminado.
  - **Plantillas y rendimiento:** copia de plantilla; rendimiento de 200 comentarios en 50 hilos.
- **db:** el trigger de compatibilidad.
- **richtext:** marca, `StripComments` y límite de comentarios.
- **Vitest:**
  - **Ficha:** talla, asignados y sección de comentarios.
  - **Comentarios y página:** hilo resuelto, marca al pegar, «Comentar» sobre una selección.
