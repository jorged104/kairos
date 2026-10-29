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

- [ ] **T3 — Almacenamiento y configuración** (`internal/files/store.go`, `internal/config`, `docker-compose.yml` profile `s3`, `Makefile`, `.env.example`, CI)
  - Depende de: T1
  - RF: RF-21, RF-22, RF-54
  - Hecho cuando: tests de `dbStore` y de `s3Store` contra MinIO (`TEST_S3_ENDPOINT`); tests de config (sin S3, completo, a medias → error); `make up-s3` levanta MinIO; `actionlint` sin avisos sobre el workflow con el servicio MinIO.

- [ ] **T4 — Subir y servir archivos** (`internal/files` handler y service)
  - Depende de: T3
  - RF: RF-14 a RF-18, RF-23, RF-24, RF-55
  - Hecho cuando: tests de detección por contenido (incluidos los rechazos de SVG, HTML, ejecutable y ZIP disfrazado), de 10 MB exactos y 10 MB + 1, y `apitest` de subir y servir con cabeceras, lector sin acceso con 404 y 25 archivos en una página.

- [ ] **T5 — Guardar la página** (`internal/pages`, `description` opcional en los `PATCH`)
  - Depende de: T2, T4
  - RF: RF-34, RF-36, RF-43, RF-44, RF-47, RF-49, RF-50, RF-59
  - Hecho cuando: `apitest` de `PUT …/page` en los tres dueños, `page_conflict`, cambio de estado sin conflicto, `PATCH` sin `description` que la conserva, `PATCH` antiguo que incrementa `page_version`, marcar y desmarcar archivos, descripción de la 004 leída y guardada, y auditoría `description_changed`.

- [ ] **T6 — Borrados y plantillas** (`workitems`, `milestones`, `projects/templates.go`)
  - Depende de: T5
  - RF: RF-51 a RF-53
  - Hecho cuando: `apitest` de archivos marcados al borrar un work item y un hito; la copia desde plantilla duplica los archivos (en la BD y en MinIO), reescribe los `fileId`, y marcar uno de la copia no afecta a la plantilla.

- [ ] **T7 — Menciones** (`internal/mentions`, `refs` en los `GET`)
  - Depende de: T5
  - RF: RF-25 a RF-30
  - Hecho cuando: `apitest` de `/api/mentions` filtrado por ámbito y de `refs` con `ok`, `no_access` (sin `label`) y `deleted`; el test de matriz cubre las rutas nuevas.

- [ ] **T8 — Dependencias de frontend y extensiones de página** (`package.json`, `features/richtext/pageExtensions.ts`)
  - Depende de: T2
  - RF: RF-1, RF-2, RF-5, RF-6, RF-56, RF-57
  - Hecho cuando: dependencias instaladas con versión exacta; `pageExtensions.spec` compara los nodos con la lista del backend y prueba los atajos de Markdown; `npm run build` en verde.

- [ ] **T9 — Editor de páginas** (`PageEditor.vue`, `PageView.vue`, `SlashMenu.vue`, `MentionList.vue`, `FileBlock.vue`)
  - Depende de: T8
  - RF: RF-3, RF-4, RF-7 a RF-13, RF-19, RF-20, RF-25 a RF-28
  - Hecho cuando: specs de Vitest del menú `/` (filtrar, flechas, Enter, Escape), del menú del bloque, del modo lectura, de `FileBlock` (progreso, error, reintento) y de las menciones `sin acceso` y `eliminado`.

- [ ] **T10 — Autoguardado** (`features/pages/usePageAutosave.ts`)
  - Depende de: T9
  - RF: RF-31 a RF-36
  - Hecho cuando: spec con temporizadores falsos (2 s, al salir, conflicto con aviso, reintento, límite de tamaño); medido el tamaño gzip del bloque de la página y anotado aquí (ING-3: más de +150 KB → asa propia).

- [ ] **T11 — Vistas** (`WorkItemSheet.vue`, panel en `ProjectWorkItems.vue`, `WorkItemPage.vue`, layout, `ProjectPage.vue`, `MilestonePage.vue`)
  - Depende de: T10
  - RF: RF-37 a RF-43
  - Hecho cuando: specs del panel con `?item=`, Escape que cierra, «Abrir como página», ancho completo recordado (también sin `localStorage`), y páginas de proyecto e hito editables.

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
