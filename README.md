# OpenScum

Gestor de proyectos con gamificación. El trabajo se organiza en una jerarquía:

```
Objetivo → Proyecto → Hito → Actividad → Subactividad
```

Cada miembro acumula **XP** y **Monedas** al completar tareas. Todos los puntos se registran en un ledger inmutable y el nivel se calcula a partir del XP acumulado; nunca se guarda como un valor editable.

## Stack

| Capa | Tecnología |
|---|---|
| Frontend | Vue 3, Vite, TypeScript, Vue Router, Pinia, TanStack Query, vue-i18n |
| UI | Tailwind CSS v4, shadcn-vue (Reka UI), Lucide |
| Backend | Go, Chi, pgx/v5, sqlc, slog |
| Base de datos | PostgreSQL 17, migraciones con Goose |
| Infra | Docker + Docker Compose (contenedores Linux) |

## Requisitos

- Docker con Compose v2
- Go ≥ 1.24
- Node ≥ 22
- `make` (en Windows: `winget install ezwinports.make`, o trabajar desde WSL)

## Puesta en marcha

```bash
cp .env.example .env
make up        # BD + API + frontend; espera hasta 60 s a que la BD esté sana
make migrate   # aplica las migraciones (nunca se aplican solas al arrancar)
```

Después abre http://localhost:5173. La pantalla de estado muestra si la API y la base de datos están disponibles y se actualiza sola cada 30 s.

| Servicio | URL |
|---|---|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| PostgreSQL | `localhost:5432` |

## Comandos

```bash
make up          # BD + API + frontend en contenedores
make db          # solo PostgreSQL
make api         # solo la API (sin BD disponible, la API sale con error)
make web         # solo el frontend
make down        # para y elimina los contenedores
make migrate     # aplica migraciones Goose
make sqlc        # regenera el código de sqlc (en contenedor)
make test        # tests de backend y frontend (test-api / test-web por separado)
make dev-api     # API en el host contra la BD en contenedor
make dev-web     # Vite en el host
```

## API

| Ruta | Respuesta |
|---|---|
| `GET /health/live` | `200` mientras el proceso está vivo; no consulta la BD |
| `GET /health/ready` | `200` si la BD responde en 2 s; si no, `503` con `database_unavailable` |

Los errores siguen siempre el formato `{ "error": { "code": "...", "message": "..." } }`.

## Cómo se trabaja en este repo

El desarrollo está guiado por especificaciones: nada se implementa si no está en una spec aprobada.

1. **Spec** (`docs/specs/NNN-nombre/spec.md`): define qué se construye y por qué, con requisitos en notación EARS. Se redacta con la skill `spec-generator`.
2. **Plan** (`plan.md`): recoge las decisiones técnicas.
3. **Tareas** (`task.md`): pasos en orden de dependencia, con los RF que cubre cada uno y un criterio de «hecho».
4. **Implementación** en una rama `feat/NNN-descripcion`, con un commit por tarea y los tests en verde.

| Spec | Estado |
|---|---|
| [001 — Infraestructura base](docs/specs/001-infraestructura-base/spec.md) | Implementada en `feat/001-infraestructura-base` |

Las reglas para agentes de código (Claude Code, Codex, Copilot…) están en [AGENTS.md](AGENTS.md).
