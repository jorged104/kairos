# Plan 005 — Páginas tipo Notion para work items, proyectos e hitos

Plan de implementación de [spec.md](spec.md) (aprobada 2026-10-29). Aquí van las decisiones
técnicas; el comportamiento lo fija la spec.

## Punto de partida

- **Spec 004** en `main`: `description jsonb` + `description_text` en `projects`, `milestones`,
  `work_items` y `objectives`; `internal/richtext.Clean` valida el JSON de TipTap con lista
  blanca y lo reconstruye ([ADR 0003](../../adr/0003-descripciones-json-con-lista-blanca.md)).
  El frontend usa TipTap 3.31.4 (`RichTextEditor.vue`, `RichTextView.vue`, `extensions.ts`).
- El `PATCH` de work items, hitos y proyectos reemplaza todos los campos, también la
  descripción, y compara `version` con la del elemento completo.
- No hay subida de archivos ni almacenamiento de objetos.
- Los proyectos no se borran (spec 004: ciclo de vida). RF-51 aplica a work items e hitos.

## Dependencias nuevas

Todas en la versión exacta ya usada por TipTap en el proyecto (3.31.4) salvo indicación. Se
añaden a «Decisiones» de la spec en el mismo commit que este plan.

| Paquete | Uso | Por qué |
|---|---|---|
| `@tiptap/extension-drag-handle-vue-3`, `@tiptap/extension-drag-handle`, `@tiptap/extension-node-range` | Asa para arrastrar bloques (RF-10, RF-11) | Componente oficial para Vue 3. Arrastrar en ProseMirror (selección de nodo, posición de suelta, anidados) es la parte más delicada del editor. |
| `@tiptap/extension-collaboration`, `@tiptap/y-tiptap`, `yjs` | Solo porque `extension-drag-handle` los importa | Peer obligatorio de la anterior (sin `peerDependenciesMeta` opcional). No se usa colaboración (fuera de alcance). Coste medido en T10; si el bloque de la página supera +150 KB gzip, se cambia por un asa propia (ING-3). |
| `@tiptap/suggestion` | Menú `/` y buscadores `@`/`#` (RF-7 a RF-9, RF-25, RF-26) | Base oficial de los dos. |
| `@tiptap/extension-mention` | Nodo de mención (RF-6, RF-27) | Admite varios disparadores (`@`, `#`) en v3. |
| `@tiptap/extension-details` (+ peer `@tiptap/extension-text-style`) | Bloques plegables (RF-2, RF-4, RF-57) | Oficial; el peer no se importa en tiempo de ejecución. |
| `@tiptap/extension-image` | Bloque de imagen (RF-14) | Oficial; se extiende con `fileId`. |
| `@floating-ui/dom` (versión ya instalada en `node_modules`, fijada exacta) | Posicionar menús | Peer de `suggestion`; hoy entra como transitiva de reka-ui. |
| Go: `github.com/minio/minio-go/v7` | Cliente S3 (RF-21) | Funciona con AWS S3 y MinIO; más pequeño que `aws-sdk-go-v2`. |

Listas de tareas: `TaskList`/`TaskItem` vienen en `@tiptap/extension-list`, ya instalado con
StarterKit. Texto de ayuda de página vacía: `Placeholder` de `@tiptap/extensions`, ya instalado.
Pegar y soltar archivos se hace con `editorProps.handlePaste`/`handleDrop` (unas 30 líneas), sin
`extension-file-handler`. El aviso destacado es un nodo propio (`callout`), unas 20 líneas.

## Decisiones técnicas

### Documento de la página
Sigue siendo el JSON de TipTap en la columna `description` (RF-44): las descripciones actuales
son páginas válidas sin migrar datos. Nodos nuevos en la lista blanca, **solo** para páginas:

| Nodo | Atributos | Regla |
|---|---|---|
| `taskList`, `taskItem` | `checked: bool` | RF-2, RF-3 |
| `details`, `detailsSummary`, `detailsContent` | `open` se descarta al guardar | RF-4: siempre se abre plegado |
| `callout` | ninguno | RF-2 |
| `image` | `fileId: uuid`; `alt` ≤ 200 | RF-14. `src` nunca se guarda: el visor lo forma con `/api/files/{fileId}` |
| `fileAttachment` | `fileId: uuid`; `name` y `size` los reescribe el servidor desde la BD | RF-15 |
| `mention` | `kind` ∈ `user`/`work_item`/`milestone`/`project`, `id: uuid`; `label` se descarta | RF-27: el nombre se resuelve al leer |

`richtext.Clean` (objetivos) no cambia. Nueva `richtext.CleanPage(raw, owner, files)` con la
lista ampliada. Devuelve también los `fileId` del documento. Un `fileId` que no sea un archivo
de **ese mismo** elemento → `invalid_rich_text` (evita enlazar el archivo de otro proyecto para
leerlo). La profundidad máxima sigue en 20 (RF-57, RF-58). El texto para buscar (RF-46) añade el
texto de los bloques nuevos y el nombre de los adjuntos.

### Versión de la página (RF-34, RF-59)
Columna nueva `page_version int NOT NULL DEFAULT 1` en las tres tablas y un trigger
`BEFORE UPDATE` que la incrementa cuando `description IS DISTINCT FROM OLD.description`. Así,
también un guardado de la API anterior (que solo conoce `version`) cuenta para el conflicto.

- `PUT /api/{work-items|milestones|projects}/{id}/page` con `{ page, page_version }`: compara
  solo `page_version`; si no coincide → `409 page_conflict` con la versión actual. No compara
  `version`, así un cambio de estado no interrumpe (RF-59). Sí incrementa `version` (la
  columna general), para que un `PATCH` posterior con la versión vieja siga detectando el cambio.
- El `PATCH` de los tres admite `description` **opcional** (puntero). Si no llega, se conserva
  (mismo patrón que `weight`). El frontend nuevo ya no la envía por `PATCH`. La versión anterior
  del frontend sigue mandándola y funcionando.

### Archivos (RF-14 a RF-24, RF-49 a RF-55)
Tablas nuevas:

- `page_files`: `id uuid PK`, `project_id` (FK), `owner_kind` (`work_item`|`milestone`|`project`),
  `owner_id uuid`, `kind` (`image`|`file`), `name`, `content_type`, `size_bytes`, `storage`
  (`db`|`s3`), `s3_key`, `created_by`, `created_at`, `deleted_at`. Índice
  `(owner_kind, owner_id) WHERE deleted_at IS NULL`.
- `page_file_data (file_id PK → page_files ON DELETE CASCADE, data bytea)`: el contenido cuando
  `storage = 'db'`, aparte para que los listados no lo lean.

Sin FK polimórfica: el servicio comprueba el dueño al subir y al guardar la página.

- **Subir:** `POST /api/{kind}/{id}/files` multipart, un archivo. Permiso de escritura sobre el
  elemento. Límite con `http.MaxBytesReader` a 10 MB + margen → `413 file_too_large` (RF-16).
  Tipo por contenido con `http.DetectContentType` sobre los primeros 512 bytes (RF-18). Casos:
  PNG/JPEG/GIF/WebP → `image`; PDF; ZIP, comprobando con `archive/zip` si es OOXML
  (`[Content_Types].xml` → docx/xlsx/pptx por la extensión declarada); OLE (`D0 CF 11 E0`) para
  doc/xls/ppt; texto UTF-8 válido para TXT/CSV. Lo demás, SVG y HTML incluidos (los detecta
  como texto con `<`), → `415 file_type_not_allowed` (RF-17). Devuelve
  `{ id, kind, name, size, content_type }`.
- **Almacenar (RF-21, RF-22, RF-54):** interfaz `files.Store` con `dbStore` y `s3Store`.
  `storage` y `s3_key` se guardan por archivo, así cada uno se sirve de donde se guardó aunque
  cambie la configuración. Con S3 configurado pero caído, la subida falla con
  `503 storage_unavailable` (sin alternativa a la BD).
- **Servir (RF-23, RF-24):** `GET /api/files/{id}` con sesión. Permiso de lectura del proyecto
  del dueño; si está marcado como borrado → 404. Se sirve a través de la API (también desde S3),
  sin URL prefirmadas, para que ninguna URL dé acceso sin sesión. Cabeceras: `Content-Type` el
  detectado, `X-Content-Type-Options: nosniff`,
  `Content-Security-Policy: default-src 'none'; sandbox`, `Cache-Control: private, max-age=3600`.
  Imágenes con `Content-Disposition: inline`; el resto `attachment; filename*=UTF-8''…`.
- **Marcar como borrado (RF-49 a RF-52):** al guardar la página, en la misma transacción:
  `UPDATE page_files SET deleted_at = now()` para los del dueño que no están en el documento, y
  `deleted_at = NULL` para los que sí (deshacer, RF-50). Al borrar un work item o un hito se
  marcan todos los suyos (RF-51). Nada borra filas ni objetos (RF-52). Los archivos subidos y
  nunca guardados en la página quedan sin marcar hasta el siguiente guardado de esa página. El
  mantenimiento de la spec posterior los tratará.
- **Plantillas (RF-53):** tras `CopyWorkItem` y la copia de hitos y proyecto, por cada página con
  archivos: copia de la fila y del contenido (`INSERT … SELECT` para `db`, `CopyObject` para
  `s3`) con id nuevo, y se reescriben los `fileId` del JSON copiado. Con S3 la copia de objetos
  ocurre antes de confirmar la transacción. Si la transacción falla, quedan objetos huérfanos en
  S3, que también recogerá el mantenimiento.
- **Sin límite de número (RF-55).**

### Menciones (RF-25 a RF-30)
- Buscador `@`: el directorio de la spec 003 (`GET /api/users?q=` con estado activo), el mismo que usa el selector de asignado.
- Buscador `#`: nuevo `GET /api/mentions?q=` que devuelve work items por clave o título, hitos y
  proyectos visibles para quien busca. Squirrel sobre las tres tablas con el filtro de ámbito de
  la spec 004, máximo 10 por grupo.
- Al leer una página, la API añade `refs`: `{ "<id>": { "label": "KAI-8 Autenticación",
  "state": "ok" | "no_access" | "deleted" } }`, calculado en una consulta por tipo.
  `no_access` nunca lleva `label` (RF-28). El visor pinta la mención solo desde `refs`.
- Ningún evento ni aviso (RF-30).

### Configuración
Variables opcionales en `.env.example` (sin valores reales):
`FILES_S3_ENDPOINT`, `FILES_S3_BUCKET`, `FILES_S3_REGION`, `FILES_S3_ACCESS_KEY`,
`FILES_S3_SECRET_KEY`, `FILES_S3_USE_SSL` (por defecto `true`). Con `FILES_S3_ENDPOINT` vacío
→ base de datos (RF-22). Si está a medias (endpoint sin bucket o sin claves), la API no arranca.
Al arrancar con S3 se comprueba que el bucket existe; si no existe, no arranca. `docker-compose`
gana un profile `s3` con MinIO y un `make up-s3`. La previsualización no cambia: sin estas
variables, guarda en la BD.

## Esquema (migración `00005_paginas.sql`, solo aditiva)

- `page_version int NOT NULL DEFAULT 1` en `projects`, `milestones` y `work_items`, y una
  función de trigger `bump_page_version()` en las tres.
- Tablas `page_files` y `page_file_data`.
- Down: elimina trigger, función, tablas y columnas.

La API anterior no lee las columnas nuevas (sqlc expande `*` al generar). Sus `PATCH` siguen
funcionando y el trigger mantiene `page_version`.

## Backend

| Paquete | Qué |
|---|---|
| `internal/richtext` | `CleanPage` y la lista ampliada; tests de cada nodo, de `open` descartado, `src` rechazado, `fileId` ajeno rechazado y texto extraído. |
| `internal/files` (nuevo) | `handler.go` (subir, servir), `service.go` (detección de tipo, permisos, marcar o desmarcar, duplicar), `store.go` (`dbStore`, `s3Store`), `types.go`. |
| `internal/pages` (nuevo) | `PUT …/page` de los tres dueños, con `refs` en la respuesta; usa `richtext.CleanPage`, `files.Sync` y `audit.Record` (`description_changed`, RF-47) en la misma transacción. |
| `internal/mentions` (nuevo) | `GET /api/mentions` y la resolución de `refs`, también usada por los `GET` de work item, hito y proyecto. |
| `workitems`, `milestones`, `projects` | `description` opcional en el `PATCH`; `page_version` y `refs` en las respuestas; marcar archivos al borrar; duplicar archivos en plantillas. |
| `config`, `cmd/api` | Carga de S3 y wiring del `files.Store`. |

### API (cambios aditivos)

| Ruta | Permiso | RF |
|---|---|---|
| `PUT /api/work-items/{id}/page`, `/api/milestones/{id}/page`, `/api/projects/{id}/page` | el de modificar el elemento | RF-31 a RF-36, RF-59 |
| `POST /api/work-items/{id}/files` (y de hitos y proyectos) | el de modificar el elemento | RF-14 a RF-20, RF-55 |
| `GET /api/files/{id}` | lectura del proyecto del dueño | RF-23, RF-24 |
| `GET /api/mentions?q=` | sesión; resultados filtrados por ámbito | RF-26 |

Errores nuevos con su `error.code` en `locales/es.json`: `page_conflict`, `file_too_large`,
`file_type_not_allowed`, `storage_unavailable`. `invalid_rich_text` ya existe. Sin permisos
nuevos en la matriz: se reutilizan los de la spec 004. El test de matriz cubre las rutas nuevas.

## Frontend

- **`features/richtext/`**
  - `pageExtensions.ts`: StarterKit + TableKit + TaskList/TaskItem + Details + Callout + Image
    (con `fileId`) + FileAttachment + Mention (`@`, `#`) + SlashCommand + Placeholder.
    `ALLOWED_NODES` de página, con un test que compara la lista con la del backend, como hoy.
  - `PageEditor.vue`: editor sin barra de herramientas; asa con `DragHandle` de
    `extension-drag-handle-vue-3` y su menú (convertir, duplicar, borrar). En pantallas táctiles
    se oculta el asa.
  - `PageView.vue`: modo lectura (RF-13), con las casillas deshabilitadas.
  - `SlashMenu.vue` y `MentionList.vue`: listas con flechas, Enter y Escape (RF-8, RF-9).
  - `FileBlock.vue`: progreso, error con reintentar y quitar (RF-19, RF-20). Se sube con
    `XMLHttpRequest` para tener progreso, con la cabecera CSRF.
- **`features/pages/usePageAutosave.ts`**
  - guarda a los 2 s sin cambios (RF-31);
  - al salir guarda antes de cerrar: `onBeforeRouteLeave`, cierre del panel y `pagehide` con
    `fetch(…, { keepalive: true })` (RF-32);
  - estado `saved | dirty | saving | conflict | error` para el indicador (RF-33);
  - con `page_conflict` deja de guardar y muestra el aviso con «Copiar mi texto» y «Recargar su
    versión» (RF-34);
  - un error de red reintenta a los 2, 4, 8… s, con un máximo de 30 s (RF-35);
  - el límite de tamaño se comprueba antes de enviar (RF-36).
- **Vistas**
  - `WorkItemSheet.vue`: contenido común de la ficha (título, propiedades y página), sacado de
    `WorkItemPage.vue`.
  - El panel lateral se abre en `ProjectWorkItems.vue` con `?item=KAI-12` en la URL (RF-37).
    Escape lo cierra (RF-42).
  - «Abrir como página» lleva a `/work-items/:wikey` (RF-38, RF-39).
  - «Ancho completo» guarda la elección en `localStorage` con try/catch y oculta la navegación
    del layout (RF-40, RF-41).
  - `ProjectPage.vue` y `MilestonePage.vue` usan `PageEditor` (RF-43).
- **Textos:** claves `page.*` (menú `/`, bloques, estados de guardado, conflicto, archivos,
  menciones «sin acceso» y «eliminado», ayuda de página vacía) y los errores nuevos (RF-48).

## Tests

- **`richtext`:** nodos nuevos aceptados; `open` descartado; `src` de imagen rechazado; `fileId`
  ajeno rechazado; 21 niveles rechazados; texto con adjuntos; objetivos siguen rechazando los
  nodos nuevos.
- **`files`:**
  - detección por contenido: PNG con extensión `.pdf` se guarda como imagen; SVG, HTML, un
    ejecutable y un ZIP renombrado `.docx` sin `[Content_Types].xml` se rechazan;
  - 10 MB exactos aceptados y 10 MB + 1 rechazados;
  - `dbStore`, y `s3Store` contra MinIO cuando hay `TEST_S3_ENDPOINT`. En CI se añade un
    servicio MinIO al job `test` de GitHub y el workflow se pasa por `actionlint`.
- **`apitest`:**
  - subir y servir con cabeceras;
  - un lector sin acceso recibe 404;
  - marcar al quitar y desmarcar al deshacer;
  - marcar al borrar un work item o un hito;
  - la plantilla duplica los archivos y borrar los de la copia no afecta a la plantilla;
  - con S3 configurado, los archivos antiguos se siguen sirviendo desde la BD;
  - conflicto de página; un cambio de estado no es conflicto (RF-59);
  - un `PATCH` sin `description` la conserva;
  - un `PATCH` con `description` de la API anterior incrementa `page_version`;
  - `refs` con `ok`, `no_access` y `deleted`;
  - `/api/mentions` filtra por ámbito;
  - auditoría `description_changed`.
- **Vitest:**
  - menú `/` (filtrar, flechas, Enter, Escape);
  - atajos de Markdown;
  - autoguardado con temporizadores falsos (2 s, al salir, conflicto, reintento);
  - modo lectura;
  - panel lateral y Escape;
  - ancho completo recordado;
  - `FileBlock` (progreso, error);
  - claves i18n.
- **Rendimiento (RNF):** página de 50 000 caracteres con 20 imágenes en el entorno local:
  pintado < 1 s (Vitest con jsdom no lo mide; se mide a mano en T14) y autoguardado < 300 ms
  (test de API con la página grande).

## Tabla RF → dónde → cómo se prueba

| RF | Dónde | Test |
|---|---|---|
| RF-1, RF-2, RF-5, RF-6, RF-57, RF-58 | `pageExtensions.ts`, `richtext.CleanPage` | `richtext_test`, `pageExtensions.spec` |
| RF-3, RF-4 | `taskItem.checked`; `details.open` descartado | `richtext_test`, `PageEditor.spec` |
| RF-7 a RF-9 | `SlashMenu.vue` | `SlashMenu.spec` |
| RF-10 a RF-12 | `PageEditor.vue` (DragHandle y menú) | `PageEditor.spec` (menú); arrastre a mano (T14) |
| RF-13 | `PageView.vue` | `PageView.spec` |
| RF-14, RF-15, RF-17, RF-18 | `files.service` (detección) | `files` tests |
| RF-16 | `MaxBytesReader` | `files` tests |
| RF-19, RF-20 | `FileBlock.vue` | `FileBlock.spec` |
| RF-21, RF-22, RF-54 | `files.Store`, `config` | `files` tests (db, MinIO), `apitest` |
| RF-23, RF-24 | `GET /api/files/{id}` | `apitest` |
| RF-25 a RF-30 | `mentions`, `MentionList.vue` | `apitest`, `MentionList.spec` |
| RF-31 a RF-36 | `usePageAutosave`, `PUT …/page` | `usePageAutosave.spec`, `apitest` |
| RF-37 a RF-42 | `ProjectWorkItems.vue`, `WorkItemSheet.vue`, layout | `ProjectWorkItems.spec`, `WorkItemPage.spec` |
| RF-43 | `ProjectPage.vue`, `MilestonePage.vue` | `apitest` (`PUT` de hito y proyecto), spec de componente |
| RF-44 | sin migración de datos | `apitest`: una descripción de la 004 se lee y se guarda |
| RF-45 | lista blanca | `richtext_test` |
| RF-46 | texto extraído | `richtext_test`, búsqueda en `apitest` |
| RF-47 | `audit.Record` | `apitest` |
| RF-48 | `locales/es.json` | test de claves i18n |
| RF-49 a RF-52 | `files.Sync`, borrado de work item e hito | `apitest` |
| RF-53 | copia de plantilla | `apitest` (`templates_test`) |
| RF-55 | sin límite | `apitest` (página con 25 archivos) |
| RF-56 | input rules de TipTap | `PageEditor.spec` |
| RF-59 | `PUT …/page` compara `page_version` | `apitest` |

## Revisión de ingeniería

- **ING-1 (cobertura):** todos los RF están en la tabla. RF-11 (arrastrar) no se puede probar
  bien en jsdom y queda en la prueba manual de T14. Resuelto.
- **ING-2 (versión):** comparar `version` en el autoguardado haría que un cambio de estado
  provoque conflicto, contra RF-59. Por eso `page_version` va en una columna propia, con un
  trigger para que la API anterior también la mantenga. Resuelto.
- **ING-3 (dependencias):** `extension-drag-handle` importa `@tiptap/extension-collaboration` y
  `@tiptap/y-tiptap` (y `yjs`) aunque no se usen. Se acepta por ser el componente oficial y
  arrastrar sin fallos es delicado. Va solo en el bloque de la página (ruta con carga diferida).
  Criterio de salida medido en T10: más de +150 KB gzip → asa propia con la selección de nodo de
  ProseMirror, sin esas tres. `extension-file-handler` se descarta porque bastan
  `handlePaste`/`handleDrop`. Abierto hasta medir en T10.
- **ING-4 (seguridad de archivos):** un `fileId` de otro elemento en el JSON permitiría leer
  archivos ajenos a través de la página propia. `CleanPage` exige que el archivo sea del mismo
  dueño. Se sirve con `nosniff`, CSP `sandbox` y `attachment` salvo imágenes. SVG queda fuera
  por ser script. Las imágenes nunca llevan `src` guardado. Resuelto.
- **ING-5 (fugas en menciones):** `refs` con `no_access` no lleva `label`, y `/api/mentions`
  aplica el ámbito en la consulta, no después. Resuelto.
- **ING-6 (datos y compatibilidad):** migración solo aditiva. `description` sigue donde está, la
  API anterior lee y escribe igual y el trigger mantiene `page_version`. Los proyectos no se
  borran, así que RF-51 solo aplica a work items e hitos. Resuelto.
- **ING-7 (tamaño en BD):** con almacenamiento en BD, 10 MB por archivo en `bytea` aparte, fuera
  de las consultas habituales. Sin límite de número (RF-55) la BD puede crecer. Es un riesgo
  aceptado por la spec, y el mantenimiento posterior lo gestionará. Resuelto (aceptado).
- **ING-8 (autoguardado al salir):** `keepalive` limita el cuerpo a 64 KB. Una página mayor al
  cerrar la pestaña puede no guardarse. Mitigación: al navegar dentro de la app (cerrar panel,
  cambiar de ruta) se espera al guardado normal sin `keepalive`. Solo el cierre de pestaña usa
  `keepalive`, y además se avisa con `beforeunload` si hay cambios. Resuelto.
- **ING-9 (simplicidad):** el buscador `@` reutiliza el directorio de usuarios. El nodo de imagen
  reutiliza `extension-image`. No hay colaboración ni historial. Resuelto.

## Riesgos y limitaciones

- Archivos subidos y nunca guardados en ninguna página, y objetos S3 de copias fallidas: quedan
  huérfanos hasta el mantenimiento de la spec posterior.
- Quitar la configuración de S3 deja sin servir los archivos guardados allí (caso límite
  aceptado en la spec).

## Tareas
Ver [tasks.md](tasks.md).
