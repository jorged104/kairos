# Tareas 005 — Páginas tipo Notion para work items, proyectos e hitos

Derivadas de [plan.md](plan.md) para [spec.md](spec.md), en orden de dependencia.
Un commit por tarea en `feat/005-paginas-tipo-notion`; ninguna se marca sin su verificación en verde.
Las tareas marcadas **(tú)** necesitan una comprobación manual tuya.

- [x] **T1 — Esquema** (`00005_paginas.sql`, queries, `make sqlc`)
  - Depende de: —
  - RF: soporte de RF-14 a RF-24, RF-34, RF-49 a RF-55, RF-59
  - Hecho cuando: la migración sube, baja y vuelve a subir en limpio; test de BD de que el trigger incrementa `page_version` solo si cambia `description`; sqlc genera sin errores; `go test ./...` en verde.

- [x] **T2 — Lista blanca de páginas** (`internal/richtext`)
  - Depende de: —
  - RF: RF-1 a RF-6, RF-45, RF-46, RF-57, RF-58
  - Hecho cuando: tests de `CleanPage` (nodos nuevos, `open` descartado, `src` rechazado, `fileId` ajeno rechazado, 21 niveles rechazados, texto con adjuntos) y de que `Clean` sigue rechazando los nodos nuevos.

- [x] **T3 — Almacenamiento y configuración** (`internal/files/store.go`, `internal/config`, `docker-compose.yml` profile `s3`, `Makefile`, `.env.example`, CI)
  - Depende de: T1
  - RF: RF-21, RF-22, RF-54
  - Hecho cuando: tests de `dbStore` y de `s3Store` contra MinIO (`TEST_S3_ENDPOINT`); tests de config (sin S3, completo, a medias → error); `make up-s3` levanta MinIO; `actionlint` sin avisos sobre el workflow con el servicio MinIO.
  - Desviación: MinIO ya no publica imágenes en Docker Hub ni Quay, así que el S3 de desarrollo y de CI es SeaweedFS 4.48, también compatible con S3. El cliente sigue siendo `minio-go`, que funciona con cualquier S3. En CI va como `docker run` en un paso, porque los servicios de Actions no admiten los argumentos de arranque que necesita.

- [x] **T4 — Subir y servir archivos** (`internal/files` handler y service)
  - Depende de: T3
  - RF: RF-14 a RF-18, RF-23, RF-24, RF-55
  - Hecho cuando: tests de detección por contenido (incluidos los rechazos de SVG, HTML, ejecutable y ZIP disfrazado), de 10 MB exactos y 10 MB + 1, y `apitest` de subir y servir con cabeceras, lector sin acceso con 404 y 25 archivos en una página.
  - Desviación: en la spec 004 todos los proyectos se leen con `work:read`, así que un usuario de otro equipo puede ver un archivo igual que ve la página que lo contiene (RF-23). El 404 queda para archivos que no existen o están marcados como borrados, y para plantillas sin `templates:use`. La subida está en el paquete `pages`, que sabe quién puede cambiar cada dueño; `files` guarda, sirve y marca.

- [x] **T5 — Guardar la página** (`internal/pages`, `description` opcional en los `PATCH`)
  - Depende de: T2, T4
  - RF: RF-34, RF-36, RF-43, RF-44, RF-47, RF-49, RF-50, RF-59
  - Hecho cuando: `apitest` de `PUT …/page` en los tres dueños, `page_conflict`, cambio de estado sin conflicto, `PATCH` sin `description` que la conserva, `PATCH` antiguo que incrementa `page_version`, marcar y desmarcar archivos, descripción de la 004 leída y guardada, y auditoría `description_changed`.
  - Nota: el `PATCH` que trae `description` (el del frontend anterior) también la valida como página y sincroniza sus archivos, para que ningún camino deje una página sin controlar sus archivos. Las altas validan la descripción como página sin archivos. La descarga de archivos pasó de `files` a `pages`, porque `projects` necesita importar `files` y `files` no puede importar `projects`.

- [x] **T6 — Borrados y plantillas** (`workitems`, `milestones`, `projects/templates.go`)
  - Depende de: T5
  - RF: RF-51 a RF-53
  - Hecho cuando: `apitest` de archivos marcados al borrar un work item y un hito; la copia desde plantilla duplica los archivos (en la BD y en MinIO), reescribe los `fileId`, y marcar uno de la copia no afecta a la plantilla.

- [x] **T7 — Menciones** (`internal/mentions`, `refs` en los `GET`)
  - Depende de: T5
  - RF: RF-25 a RF-30
  - Hecho cuando: `apitest` de `/api/mentions` filtrado por ámbito y de `refs` con `ok`, `no_access` (sin `label`) y `deleted`; el test de matriz cubre las rutas nuevas.
  - Nota: la búsqueda `#` usa consultas fijas de sqlc en vez de Squirrel, porque el filtro no cambia de forma. Un usuario dado de baja (`offboarded`) se lee como «eliminado». Los `refs` van en las lecturas de un solo elemento, en sus `PATCH` y en el guardado de la página; los listados no los llevan.

- [x] **T8 — Dependencias de frontend y extensiones de página** (`package.json`, `features/richtext/pageExtensions.ts`)
  - Depende de: T2
  - RF: RF-1, RF-2, RF-5, RF-6, RF-56, RF-57
  - Hecho cuando: dependencias instaladas con versión exacta; `pageExtensions.spec` compara los nodos con la lista del backend y prueba los atajos de Markdown; `npm run build` en verde.

- [x] **T9 — Editor de páginas** (`PageEditor.vue`, `PageView.vue`, `SlashMenu.vue`, `MentionList.vue`, `FileBlock.vue`)
  - Depende de: T8
  - RF: RF-3, RF-4, RF-7 a RF-13, RF-19, RF-20, RF-25 a RF-28
  - Hecho cuando: specs de Vitest del menú `/` (filtrar, flechas, Enter, Escape), del menú del bloque, del modo lectura, de `FileBlock` (progreso, error, reintento) y de las menciones `sin acceso` y `eliminado`.
  - Desviación: `PageView` es el mismo `PageEditor` con `editable` a falso, y `SlashMenu` y `MentionList` son un solo `SuggestionList`, que sirve para `/`, `@` y `#`. Las menciones se pintan con una vista de nodo que lee `refs` de forma reactiva, para que un nombre resuelto tras guardar se vea sin recargar. Las casillas de las listas de tareas se deshabilitan a mano en modo lectura, porque TipTap solo las revierte. El arrastre en sí (RF-11) queda para T14.

- [x] **T10 — Autoguardado** (`features/pages/usePageAutosave.ts`)
  - Depende de: T9
  - RF: RF-31 a RF-36
  - Hecho cuando: spec con temporizadores falsos (2 s, al salir, conflicto con aviso, reintento, límite de tamaño); medido el tamaño gzip del bloque de la página y anotado aquí (ING-3: más de +150 KB → asa propia).
  - Nota: el autoguardado y el editor se juntan en `PageSection`, con el indicador de estado y el aviso de conflicto («Copiar mi texto», «Recargar su versión»). El tamaño del bloque se mide en T11, cuando el editor ya está en las pantallas; antes el empaquetador lo descarta por no usarse.
  - Medido en T11 con `npm run build`: el bloque del editor pasa de 134,4 KB a 140,7 KB comprimido y la página añade un bloque nuevo de 65,6 KB, unos +72 KB en total. Está por debajo de los +150 KB de ING-3, así que se queda el asa oficial.

- [x] **T11 — Vistas** (`WorkItemSheet.vue`, panel en `ProjectWorkItems.vue`, `WorkItemPage.vue`, layout, `ProjectPage.vue`, `MilestonePage.vue`)
  - Depende de: T10
  - RF: RF-37 a RF-43
  - Hecho cuando: specs del panel con `?item=`, Escape que cierra, «Abrir como página», ancho completo recordado (también sin `localStorage`), y páginas de proyecto e hito editables.
  - Nota: el panel es un lateral fijo sobre la parte derecha del árbol, que sigue visible a la izquierda. Se abre desde la clave de la fila; el título sigue editándose en la celda. Los `PATCH` del frontend ya no envían `description`, y la ficha usa la versión de la caché, que el autoguardado mantiene al día, para no chocar con sus propios guardados.

- [ ] **T12 — Textos** (`locales/es.json`)
  - Depende de: T9, T11
  - RF: RF-48
  - Hecho cuando: el test de claves i18n pasa y ningún componente nuevo tiene texto visible fijo.

- [ ] **T13 — Rendimiento**
  - Depende de: T5, T11
  - RF: RNF de rendimiento
  - Hecho cuando: test de API de autoguardado de una página de 50 000 caracteres con 20 imágenes < 300 ms en local.

- [ ] **T14 — Prueba manual (tú)**
  - Depende de: todas
  - RF: RF-10, RF-11 y la demo de los criterios de finalización
  - Hecho cuando: la demo de la spec (pasos 1 a 6) funciona con y sin `make up-s3`, el arrastre de bloques incluido, y la página grande se pinta en < 1 s.
