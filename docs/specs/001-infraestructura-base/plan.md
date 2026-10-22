# Plan 001 — Infraestructura base

Plan de implementación de [spec.md](spec.md) (aprobada 2026-10-22).
Aquí van las decisiones técnicas; el comportamiento lo fija la spec.

## Prerrequisitos y decisiones previas

- **Herramientas en la máquina del desarrollador:** Docker con Compose v2, Go ≥ 1.24, Node ≥ 22 y `make`.
  En la máquina actual faltan **Go** y **make**: hay que instalarlos antes de la tarea 2
  (`winget install GoLang.Go` y `winget install ezwinports.make`, o trabajar desde WSL).
- **Rama:** `feat/001-infraestructura-base`. Un commit por tarea; cada tarea termina con sus tests en verde (constitución).
- **Idioma:** identificadores y código en inglés. Mensajes que ve una persona (errores al arrancar, salida del Makefile, `error.message` de la API, textos de la pantalla) en español.
- **Dependencias que NO se añaden todavía:** Squirrel, go-playground/validator, Gorilla WebSocket, TipTap, TanStack Table, VueDraggablePlus. Ninguna RF de esta spec las necesita; entran con la spec que las requiera.
- **Sin graceful shutdown ni CORS:** no los pide la spec. Se evita CORS haciendo que Vite haga de proxy hacia la API.

## Arquitectura resultante

```
navegador ──► web (Vite dev, :5173) ──proxy /health──► api (Go, :8080) ──► db (PostgreSQL 17, :5432)
```

Los tres servicios corren en contenedores Linux con Docker Compose. Las migraciones se lanzan desde el host con `make migrate` (RF-7).

### Variables de entorno (`.env.example`, copiado a `.env`)

| Variable | Uso | Valor de desarrollo |
|---|---|---|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | contenedor db | `openscum` / `openscum` / `openscum` |
| `DATABASE_URL` | API y goose desde el host | `postgres://openscum:openscum@localhost:5432/openscum?sslmode=disable` |
| `API_PORT` | puerto de la API | `8080` |
| `WEB_PORT` | puerto de Vite | `5173` |
| `API_PROXY_TARGET` | destino del proxy de Vite | `http://localhost:8080` (en contenedor: `http://api:8080`) |

Dentro de Compose, `DATABASE_URL` del servicio `api` apunta a `db:5432`. `.env` está en `.gitignore`.

### Comandos (`Makefile`)

| Comando | Qué hace | RF |
|---|---|---|
| `make db` | `docker compose up -d --wait --wait-timeout 60 db` | RF-1 |
| `make api` | `docker compose up -d --build --no-deps api` (no arrastra la BD, para poder comprobar RF-16) | RF-2 |
| `make web` | `docker compose up -d --build --no-deps web` | RF-3 |
| `make up` | `make db`; si falla, imprime «La base de datos no respondió en 60 s» y sale con 1; si no, `make api web` | RF-4, RF-5, RF-6 |
| `make down` | `docker compose down` | — |
| `make migrate` | `go tool goose -dir backend/internal/db/migrations postgres "$DATABASE_URL" up` | RF-8, RF-9, RF-10 |
| `make sqlc` | `docker run --rm -v ./backend:/src -w /src sqlc/sqlc generate` (sqlc necesita cgo; en contenedor evita instalar gcc en Windows) | — |
| `make test` | `make test-api test-web` | — |
| `make test-api` | `cd backend && go test ./...` | — |
| `make test-web` | `cd frontend && npm test -- --run` | — |
| `make dev-api` / `make dev-web` | API con `go run` / Vite en el host, contra la BD en contenedor | — |

El Makefile carga `.env` con `-include .env` + `export`.

## Backend

```
backend/
├── cmd/api/main.go              # run(ctx) error; main hace os.Exit(1) si run falla
├── internal/
│   ├── config/config.go         # Load() desde entorno; error si falta DATABASE_URL
│   ├── server/
│   │   ├── router.go            # chi: RequestID, Recoverer, requestLogger, rutas, 404/405 JSON
│   │   ├── logging.go           # middleware slog: method, path, status, duration_ms (RF-18)
│   │   └── respond.go           # WriteJSON, WriteError → {"error":{"code","message"}} (RF-17)
│   ├── health/
│   │   ├── handler.go           # GET /health/live, GET /health/ready
│   │   ├── service.go           # Ready(ctx): aplica timeout de 2 s (RF-15), configurable para tests
│   │   ├── repository.go        # Ping vía sqlc
│   │   └── types.go             # interfaz Pinger, structs de respuesta
│   └── db/
│       ├── migrations/00001_baseline.sql   # migración no-op inicial
│       ├── queries/health.sql              # -- name: Ping :one / SELECT 1;
│       └── sqlc/                           # generado
├── sqlc.yaml                    # engine postgresql, sql_package pgx/v5
├── go.mod                       # tool github.com/pressly/goose/v3/cmd/goose
└── Dockerfile                   # multistage: golang → distroless/static, CGO_ENABLED=0
```

Dependencias: `chi/v5`, `pgx/v5` (pgxpool), `slog` (stdlib, handler JSON a stdout), goose como `tool` de Go.

### Arranque (RF-16)
`run` carga config, crea `pgxpool`, hace `Ping` con timeout de 2 s. Si falla: `slog.Error("no se pudo conectar a la base de datos", ...)` y devuelve error → exit 1. El servicio `api` de Compose tiene `restart: "no"` para que el fallo sea visible.

### Contrato HTTP

| Ruta | BD responde | BD no responde / > 2 s |
|---|---|---|
| `GET /health/live` | `200 {"status":"ok"}` | `200 {"status":"ok"}` (no toca la BD, RF-12) |
| `GET /health/ready` | `200 {"status":"ok","database":"up"}` | `503 {"error":{"code":"database_unavailable","message":"La base de datos no está disponible"}}` |

Rutas inexistentes: `404 {"error":{"code":"not_found",...}}`; método no permitido: `405 {"error":{"code":"method_not_allowed",...}}`.

## Frontend

Vite + Vue 3 + TypeScript (`strict`), Tailwind CSS v4 (`@tailwindcss/vite`, sin `tailwind.config.ts`), shadcn-vue (`Card`, `Badge`, `Alert`), `lucide-vue-next`, Vue Router (una ruta `/`), Pinia (registrado, sin stores), TanStack Query, vue-i18n (`legacy: false`, locale `es`). Tests con Vitest + `@vue/test-utils` + jsdom.

```
frontend/src/
├── main.ts                       # app + router + pinia + VueQueryPlugin + i18n
├── App.vue                       # <RouterView />
├── router/index.ts
├── lib/i18n.ts                   # createI18n con locales/es.json
├── locales/es.json               # health.title, health.api.*, health.db.*, health.error.*
├── api/health.ts                 # fetchReadiness(): ReadinessState (nunca lanza)
├── features/health/
│   ├── useReadiness.ts           # useQuery: refetchInterval 30_000, refetchIntervalInBackground, retry: false
│   └── HealthStatus.vue          # pinta API, BD y alerta de error
├── pages/StatusPage.vue
└── components/ui/                # generado por shadcn-vue
```

### `fetchReadiness` (RF-19 a RF-24)
`fetch('/health/ready', { signal: AbortSignal.timeout(5000) })` y se traduce a estado:

| Resultado | `api` | `database` |
|---|---|---|
| 200 | `available` | `available` |
| 503 con `error.code = database_unavailable` | `available` | `unavailable` |
| timeout (> 5 s), error de red u otra respuesta | `unavailable` | `unknown` |

`HealthStatus.vue` muestra la `Alert` de error si `api` o `database` no están disponibles (RF-25). TanStack Query repinta sola al cambiar el dato (RF-27).

### Catálogo de textos (RF-28 a RF-30)
Todos los textos con `t('clave')` con clave literal. Test `i18n-keys.spec.ts`: recorre `src/**/*.{vue,ts}`, extrae las claves de `t('…')` / `$t('…')` y falla listando las que no existen en `es.json`.

## Docker Compose

- `db`: `postgres:17-alpine`, volumen `pgdata`, `healthcheck: pg_isready` cada 2 s.
- `api`: build `backend/`, `depends_on: db (service_healthy)` (solo actúa con `make up`), `restart: "no"`, puerto `${API_PORT}`.
- `web`: build `frontend/` (imagen Node que ejecuta `vite --host`), `API_PROXY_TARGET=http://api:8080`, puerto `${WEB_PORT}`. Solo modo desarrollo; build de producción fuera de alcance.

## Tareas (un commit cada una)

1. **Base del entorno:** `.env.example`, `docker-compose.yml` (solo `db`), `Makefile` con `db`, `down`. Verificación: `make db` termina con la BD sana. — RF-1
2. **Esqueleto Go:** `go.mod`, `config`, `server` (router, `respond`, logging), `main` con fallo al arrancar sin BD. Tests: formato de error en 404/405 (RF-17), log con método/ruta/código/duración (RF-18), `run` devuelve error con BD inaccesible (RF-16). — RF-16 a RF-18
3. **Endpoints de salud:** `sqlc.yaml`, `queries/health.sql`, código sqlc generado, paquete `health`. Tests con `Pinger` falso: live no llama al pinger (RF-11, RF-12), ready OK (RF-13), ready con error → 503 (RF-14), ready con pinger lento → 503 tras el timeout inyectado (RF-15). — RF-11 a RF-15
4. **Migraciones:** goose como tool, `00001_baseline.sql`, `make migrate`. Verificación manual: dos ejecuciones seguidas (la segunda sin cambios) y con la BD parada (exit ≠ 0). — RF-7 a RF-10
5. **API en contenedor:** `backend/Dockerfile`, servicio `api`, `make api`. Verificación: con la BD parada `make api` deja el contenedor en `exited (1)`. — RF-2, RF-16
6. **Esqueleto Vue:** Vite, TS strict, Tailwind v4, shadcn-vue, Router, Pinia, Query, vue-i18n, Vitest, `es.json`, test de claves. — RF-28 a RF-30
7. **Pantalla de estado:** `fetchReadiness`, `useReadiness`, `HealthStatus`, `StatusPage`. Tests: mapeo de los tres casos con `fetch` simulado (RF-20 a RF-24), alerta visible (RF-25), nueva consulta a los 30 s con timers falsos (RF-26, RF-27). — RF-19 a RF-27
8. **Web en contenedor y arranque unificado:** `frontend/Dockerfile`, servicio `web`, `make web`, `make up` con espera de 60 s. — RF-3 a RF-6
9. **Documentación:** actualizar `AGENTS.md` (comandos `db/api/web/down/test-api/test-web`, quitar `tailwind.config.ts` del árbol, Tailwind v4, `.env.example`). Ejecutar la demo de los criterios de finalización de la spec.

## Verificación final
- `make test` en verde (backend y frontend).
- Demo manual de la spec, pasos 0–5, desde un clon limpio con `cp .env.example .env`.

## Notas
- `docs/constitution.md` apareció sin commitear y parece copiada de otro proyecto (título «habits-cli», principio «Simplicidad primero» sin texto). Sus reglas (la spec manda, tests como puerta, idioma) son compatibles con este plan; conviene corregirla antes de la spec 002.
