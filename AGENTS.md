# AGENTS.md

Guía para agentes de código (Claude Code, Codex, Copilot, etc.) que trabajen en este repositorio.

## Qué es este proyecto

Sistema de gestión de proyectos con gamificación. La jerarquía de trabajo es:

```
Objetivo → Proyecto → Hito → Actividad → Subactividad
```

Cada miembro tiene **XP** y **Monedas**. Completar una tarea genera XP (y opcionalmente monedas) a través de un ledger inmutable. El nivel se deriva del XP acumulado; nunca se almacena como valor editable.

## Reglas

* Lee `docs/constitution.md` y la spec activa en `docs/specs/` antes de tocar código.
* No añadas dependencias ni cambies el formato del JSON sin actualizar antes la spec.
* No modifiques archivos dentro de `docs/specs/` salvo petición explícita.
* Toda mutación de puntos pasa por `point_ledger`. No actualices XP ni monedas directamente en `users`.
* La asignación de XP se dispara en el backend (cambio de estado a `completed`), nunca desde el frontend.
* El acceso a base de datos va por sqlc para queries fijas y Squirrel solo para filtros dinámicos. No escribas SQL en strings dentro de handlers.
* Los cambios de esquema se hacen con migraciones Goose. No edites migraciones ya aplicadas; crea una nueva.
* Cada migración debe dejar funcionando la versión anterior de la API (la vuelta atrás de previsualización no deshace migraciones): solo cambios aditivos; borrar o renombrar columnas/tablas se hace en dos despliegues (primero el código deja de usarlas, después se eliminan).
* Commits pequeños y atómicos. Un commit por cambio lógico.

## Stack

| Capa | Tecnología |
|---|---|
| Frontend | Vue 3, Vite, TypeScript, Vue Router, Pinia, TanStack Query, vue-i18n |
| UI | Tailwind CSS v4, shadcn-vue (Reka UI), Lucide (`@lucide/vue`) |
| Editor / tablas / DnD | TipTap, TanStack Table, VueDraggablePlus |
| Backend | Go, Chi, pgx/v5, sqlc, Squirrel, go-playground/validator, slog |
| Tiempo real | Gorilla WebSocket |
| Base de datos | PostgreSQL (relacional + JSONB), migraciones con Goose |
| Infra | Docker + Docker Compose |

## Estructura del proyecto

```
.
├── AGENTS.md
├── .env.example               # copiar a .env; nunca credenciales reales
├── docker-compose.yml         # entorno de desarrollo local
├── Makefile
├── .github/workflows/
│   └── preview.yml            # tests + despliegue de previsualización en push a main
├── deploy/preview/            # compose, deploy.sh y guía del servidor de previsualización
├── docs/
│   ├── constitution.md        # principios no negociables del proyecto
│   ├── specs/                 # specs funcionales (solo lectura para agentes), una carpeta por spec
│   │   ├── 001-infraestructura-base/      # spec.md, plan.md, task(s).md
│   │   ├── 002-entorno-previsualizacion/
│   │   ├── 003-usuarios/                  # auth local/OIDC, roles, permisos, equipos, atributos
│   │   ├── 004-proyectos/                 # proyectos, metodologías, hitos, work items, objetivos (OKR)
│   │   └── ...                            # gamificación y siguientes: próximos números libres
│   └── adr/                   # decisiones de arquitectura
├── backend/
│   ├── cmd/
│   │   └── api/
│   │       └── main.go        # arranque, wiring de dependencias
│   ├── internal/
│   │   ├── config/            # carga de env/config
│   │   ├── server/            # router Chi, middlewares, rutas
│   │   ├── health/            # /health/live y /health/ready
│   │   ├── apperr/            # errores de dominio con status y código; Decode + validación
│   │   ├── authz/             # roles, permisos y la matriz única rol → permisos
│   │   ├── auth/              # sesión (cookie JWT), CSRF, login local, OIDC (BFF), rate limit
│   │   ├── audit/             # registro de cambios en la misma transacción y consulta
│   │   ├── users/             # alta, edición, estados, roles y equipos de usuarios; directorio
│   │   ├── teams/             # equipos jerárquicos y líderes
│   │   ├── attributes/        # atributos dinámicos de usuario
│   │   ├── idpmappings/       # homologación de roles y grupos del IdP
│   │   ├── catalog/           # metodologías y fases, tipos, estados, prioridades y flujos
│   │   ├── customfields/      # campos personalizados de proyectos y work items
│   │   ├── fields/            # tipos de valor compartidos por atributos y campos personalizados
│   │   ├── richtext/          # validación con lista blanca de las descripciones (JSON de TipTap)
│   │   ├── events/            # eventos de dominio: outbox en la transacción + dispatcher
│   │   ├── workcore/          # piezas comunes de la spec 004 (fechas, responsables)
│   │   ├── projects/          # proyectos, jerarquía, ciclo de vida, fases y plantillas
│   │   ├── milestones/        # hitos, work items heredados y avance
│   │   ├── workitems/         # work items: composición, derivados, flujos, dependencias
│   │   ├── objectives/        # objetivos y resultados clave
│   │   ├── gamification/      # ledger, reglas de XP/monedas, niveles
│   │   ├── realtime/          # hub WebSocket
│   │   ├── apitest/           # tests HTTP de la API completa contra una BD temporal e IdP simulado
│   │   └── db/
│   │       ├── store.go       # pool + queries sqlc + InTx
│   │       ├── dbtest/        # crea una BD por test desde TEST_DATABASE_URL
│   │       ├── migrations/    # Goose: NNNN_descripcion.sql (embebidas para los tests)
│   │       ├── queries/       # .sql de entrada para sqlc
│   │       └── sqlc/          # código generado (no editar a mano)
│   ├── pkg/                   # utilidades reutilizables sin dependencias internas
│   │   └── httpx/             # WriteJSON / WriteError (formato de error común)
│   ├── sqlc.yaml
│   ├── go.mod
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── main.ts
│   │   ├── App.vue
│   │   ├── router/
│   │   ├── stores/            # Pinia
│   │   ├── api/               # clientes HTTP + hooks de TanStack Query
│   │   ├── components/
│   │   │   ├── ui/            # shadcn-vue generado
│   │   │   └── ...
│   │   ├── features/          # un directorio por dominio
│   │   │   ├── catalog/       # administración del catálogo de trabajo
│   │   │   ├── projects/      # pestañas de la ficha de proyecto
│   │   │   ├── workitems/     # árbol de work items
│   │   │   ├── richtext/      # editor y visor TipTap
│   │   │   ├── work/          # composables y piezas comunes de la spec 004
│   │   │   ├── kanban/
│   │   │   └── gamification/
│   │   ├── layouts/
│   │   ├── pages/
│   │   ├── composables/
│   │   ├── lib/
│   │   ├── locales/           # catálogos i18n, un fichero por idioma (es.json, ...)
│   │   └── types/             # tipos compartidos con la API
│   ├── index.html
│   ├── vite.config.ts         # Tailwind v4 vía plugin (sin tailwind.config.ts), proxy /health, Vitest
│   ├── components.json        # configuración de shadcn-vue
│   ├── package.json
│   └── Dockerfile
└── scripts/                   # seeds, utilidades de desarrollo
    └── keycloak/              # realm-export.json del IdP de prueba (make up-sso)
```

Cada módulo de `backend/internal/<dominio>/` sigue el mismo patrón: `handler.go` (HTTP), `service.go` (lógica), `repository.go` (acceso a datos vía sqlc/Squirrel), `types.go`.

## Flujo de trabajo esperado

1. Identifica la spec que cubre la tarea en `docs/specs/NNN-<nombre>/spec.md`. Si no existe, detente y pide que se cree (skill `spec-generator`, plantilla en `.claude/skills/spec-generator/spec-template.md`).
2. Si la tarea requiere cambio de esquema, crea primero la migración Goose y regenera sqlc (`make sqlc`).
3. Implementa backend → prueba con `make test` → implementa frontend.
4. Actualiza `docs/adr/` si tomaste una decisión arquitectónica relevante.

## Comandos

Requisitos en el host: Docker con Compose v2, Go ≥ 1.24, Node ≥ 22 y `make`. Antes de nada: `cp .env.example .env`.

```bash
make up          # BD (espera hasta 60 s a que esté sana) + API + frontend en contenedores
make up-sso      # como up, más Keycloak de prueba en localhost:8180 (profile sso)
make db          # solo PostgreSQL
make api         # solo la API (no arranca la BD; sin BD, la API sale con error)
make web         # solo el frontend (Vite, http://localhost:5173)
make down        # para y elimina los contenedores
make migrate     # aplica migraciones Goose (nunca se aplican al arrancar)
make sqlc        # regenera código de sqlc (se ejecuta en contenedor)
make test        # tests de backend y frontend (test-api / test-web por separado)
                 # los de integración necesitan TEST_DATABASE_URL (en .env); sin ella se omiten salvo en CI
make dev-api     # API en el host contra la BD en contenedor
make dev-web     # Vite en el host
```

## Previsualización

Cada push a `main` pasa los tests en GitHub y se despliega en un servidor doméstico con un runner self-hosted (`deploy/preview/`, spec 002). Puesta en marcha y operación en [deploy/preview/README.md](deploy/preview/README.md).

* El repo es público y los logs de Actions también: nunca escribas el hostname de previsualización ni credenciales en el repo, en el workflow ni en la salida de `deploy.sh`. Viven solo en `~/kairos/preview.env` en el servidor.
* El workflow que usa el runner self-hosted solo se dispara con `push` a `main`. No añadas disparadores `pull_request` que lo usen.
* Si cambias `deploy.sh`, pásale `shellcheck`; si cambias el workflow, `actionlint` (ambos en contenedor, ver `task.md` de la spec 002).

## Convenciones

* Go: `gofmt`, `go vet`, errores envueltos con `fmt.Errorf("...: %w", err)`, logging estructurado con `slog`.
* TypeScript: `strict: true`, sin `any`, componentes en `<script setup lang="ts">`.
* API: JSON en `snake_case`, respuestas de error con `{ "error": { "code": "...", "message": "..." } }`.
* i18n: ningún texto visible fijo en componentes; todo sale de `frontend/src/locales/<idioma>.json` con claves jerárquicas por dominio (`health.api.available`). Idioma por defecto: español (`es`). El frontend traduce los errores de la API a partir de `error.code`, no muestra `error.message` al usuario.
* Nombres de tablas en plural y `snake_case`; claves primarias `uuid`.
* Ramas: `feat/<spec-id>-<descripcion>`, `fix/<descripcion>`.
* Autorización: los endpoints comprueban permisos (`authz`), nunca nombres de rol. Un permiso nuevo se añade a la matriz única de `internal/authz` y al test de matriz (`apitest/matrix_test.go` o `apitest/matrix_work_test.go`).
* Toda mutación de usuarios, roles, equipos, atributos u homologaciones, y de catálogo, proyectos, fases, hitos, work items, dependencias u objetivos, se audita con `audit.Record` dentro de la misma transacción. Las descripciones solo como `description_changed`.
* Los eventos de dominio (`work_item.closed`, `milestone.completed`…) se emiten con `events.Emit` dentro de la transacción del cambio, nunca después ([ADR 0002](docs/adr/0002-eventos-de-dominio-con-outbox.md)).
* Las descripciones son documentos JSON de TipTap validados por `richtext.Clean`; nunca se guarda ni se pinta HTML ([ADR 0003](docs/adr/0003-descripciones-json-con-lista-blanca.md)).
* Las modificaciones de proyectos, hitos, work items, objetivos y resultados clave llevan `version` y responden `version_conflict` si cambió.