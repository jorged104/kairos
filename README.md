# Kairos

Gestor de proyectos con gamificación. El objetivo es organizar el trabajo en una jerarquía:

```
Objetivo → Proyecto → Hito → Actividad → Subactividad
```

Cada miembro acumulará **XP** y **Monedas** al completar tareas. Todos los puntos se registran en un ledger inmutable y el nivel se calcula a partir del XP acumulado; nunca se guarda como un valor editable.

## Estado del proyecto

El proyecto está en su fase inicial: la base técnica está lista y las funcionalidades de producto aún no.

**Disponible hoy**
- Entorno de desarrollo completo en contenedores (BD, API y frontend) con un solo comando.
- Migraciones de base de datos con Goose.
- Endpoints de salud de la API y una pantalla que muestra si la API y la base de datos responden.
- Despliegue automático de previsualización en cada push a `main`.

**Pendiente**
- Usuarios, roles, permisos y equipos (spec 003, en borrador).
- Autenticación.
- Jerarquía de trabajo (objetivos, proyectos, hitos, actividades).
- Gamificación (XP, monedas, niveles).
- Tiempo real.

## Stack

| Capa | Tecnología |
|---|---|
| Frontend | Vue 3, Vite, TypeScript, Vue Router, Pinia, TanStack Query, vue-i18n |
| UI | Tailwind CSS v4, shadcn-vue (Reka UI), Lucide |
| Backend | Go, Chi, pgx/v5, sqlc, slog |
| Base de datos | PostgreSQL 17, migraciones con Goose |
| Infra | Docker + Docker Compose (contenedores Linux) |
| CI / despliegue | GitHub Actions, runner self-hosted, Traefik |

## Requisitos

- Docker con Compose v2
- Go ≥ 1.27 (lo exige `go.mod`; con Go ≥ 1.21 la toolchain correcta se descarga sola)
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

## Previsualización

Cada push a `main` pasa los tests en GitHub Actions y, si están en verde, se despliega en el servidor de previsualización; si la nueva versión no responde, vuelve sola a la anterior. Puesta en marcha y operación en [deploy/preview/README.md](deploy/preview/README.md).

## Cómo se trabaja en este repo

El desarrollo está guiado por especificaciones: nada se implementa si no está en una spec aprobada.

El trabajo empieza con un **issue en GitHub**. La skill `issue-a-pr` (`/issue-a-pr N`) lo clasifica y prepara los documentos de abajo, con una revisión de QA de la spec y otra de ingeniería del plan, y abre una PR en borrador. Nada se implementa hasta que alguien con permiso de escritura responde `/aprobar` en la PR. Los bugs y las tareas técnicas van por un camino corto: sin spec, con el plan en la propia PR.

1. **Spec** (`docs/specs/NNN-nombre/spec.md`): define qué se construye y por qué, con requisitos en notación EARS. Se redacta con la skill `spec-generator`.
2. **Plan** (`plan.md`): recoge las decisiones técnicas.
3. **Tareas** (`task.md`): pasos en orden de dependencia, con los RF que cubre cada uno y un criterio de «hecho».
4. **Implementación** en una rama `feat/NNN-descripcion`, con un commit por tarea y los tests en verde.

| Spec | Estado |
|---|---|
| [001 — Infraestructura base](docs/specs/001-infraestructura-base/spec.md) | Implementada (#1) |
| [002 — Entorno de previsualización](docs/specs/002-entorno-previsualizacion/spec.md) | Implementada y desplegada (#2); verificación final en el servidor (T7) pendiente |
| 003 — Usuarios, roles, permisos y equipos | Borrador en la rama `feat/003-usuarios` |

Las reglas para agentes de código (Claude Code, Codex, Copilot…) están en [AGENTS.md](AGENTS.md).
