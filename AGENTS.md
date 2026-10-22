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
├── docker-compose.yml
├── Makefile
├── docs/
│   ├── constitution.md        # principios no negociables del proyecto
│   ├── specs/                 # specs funcionales (solo lectura para agentes), una carpeta por spec
│   │   ├── 001-auth/spec.md
│   │   ├── 002-work-hierarchy/spec.md
│   │   ├── 003-gamification/spec.md
│   │   └── ...
│   └── adr/                   # decisiones de arquitectura
├── backend/
│   ├── cmd/
│   │   └── api/
│   │       └── main.go        # arranque, wiring de dependencias
│   ├── internal/
│   │   ├── config/            # carga de env/config
│   │   ├── server/            # router Chi, middlewares, rutas
│   │   ├── health/            # /health/live y /health/ready
│   │   ├── auth/              # JWT, hashing, middleware de auth
│   │   ├── users/
│   │   ├── objectives/
│   │   ├── projects/
│   │   ├── milestones/
│   │   ├── workitems/         # actividades y subactividades (parent_id)
│   │   ├── gamification/      # ledger, reglas de XP/monedas, niveles
│   │   ├── realtime/          # hub WebSocket
│   │   └── db/
│   │       ├── migrations/    # Goose: NNNN_descripcion.sql
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
│   │   │   ├── auth/
│   │   │   ├── objectives/
│   │   │   ├── projects/
│   │   │   ├── workitems/
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
make db          # solo PostgreSQL
make api         # solo la API (no arranca la BD; sin BD, la API sale con error)
make web         # solo el frontend (Vite, http://localhost:5173)
make down        # para y elimina los contenedores
make migrate     # aplica migraciones Goose (nunca se aplican al arrancar)
make sqlc        # regenera código de sqlc (se ejecuta en contenedor)
make test        # tests de backend y frontend (test-api / test-web por separado)
make dev-api     # API en el host contra la BD en contenedor
make dev-web     # Vite en el host
```

## Convenciones

* Go: `gofmt`, `go vet`, errores envueltos con `fmt.Errorf("...: %w", err)`, logging estructurado con `slog`.
* TypeScript: `strict: true`, sin `any`, componentes en `<script setup lang="ts">`.
* API: JSON en `snake_case`, respuestas de error con `{ "error": { "code": "...", "message": "..." } }`.
* i18n: ningún texto visible fijo en componentes; todo sale de `frontend/src/locales/<idioma>.json` con claves jerárquicas por dominio (`health.api.available`). Idioma por defecto: español (`es`). El frontend traduce los errores de la API a partir de `error.code`, no muestra `error.message` al usuario.
* Nombres de tablas en plural y `snake_case`; claves primarias `uuid`.
* Ramas: `feat/<spec-id>-<descripcion>`, `fix/<descripcion>`.