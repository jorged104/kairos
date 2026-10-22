# Tareas 001 — Infraestructura base

Derivadas de [plan.md](plan.md) para [spec.md](spec.md), en orden de dependencia.
Un commit por tarea en `feat/001-infraestructura-base`; ninguna se marca sin sus tests en verde.

- [ ] **T0 — Prerrequisitos**
  - Depende de: —
  - RF: —
  - Hecho cuando: `go version` (≥ 1.24), `make --version`, `node -v` (≥ 22) y `docker compose version` responden, y existe la rama `feat/001-infraestructura-base`.

- [ ] **T1 — Base del entorno** (`.env.example`, `docker-compose.yml` con `db`, `Makefile` con `db` y `down`)
  - Depende de: T0
  - RF: RF-1
  - Hecho cuando: tras `cp .env.example .env`, `make db` termina con exit 0 y `docker compose ps` muestra `db` como `healthy`.

- [ ] **T2 — Esqueleto Go** (`go.mod`, `config`, `server` con router, `respond` y logging, `main` con `run`)
  - Depende de: T1
  - RF: RF-16, RF-17, RF-18
  - Hecho cuando: `make test-api` pasa con tests de 404/405 en formato `{"error":{...}}`, log con método/ruta/código/duración, y `run` devolviendo error con la BD inaccesible.

- [ ] **T3 — Endpoints de salud** (`sqlc.yaml`, `queries/health.sql`, código sqlc, paquete `health`)
  - Depende de: T2
  - RF: RF-11, RF-12, RF-13, RF-14, RF-15
  - Hecho cuando: `make sqlc` regenera sin cambios y `make test-api` pasa con tests de live sin llamar al pinger, ready 200, ready 503 por error y ready 503 por timeout.

- [ ] **T4 — Migraciones** (goose como tool, `00001_baseline.sql`, `make migrate`)
  - Depende de: T1, T2
  - RF: RF-7, RF-8, RF-9, RF-10
  - Hecho cuando: `make migrate` aplica la baseline, una segunda ejecución termina en 0 sin cambios, con la BD parada termina con exit ≠ 0 y mensaje de error, y la API arranca sin migrar.

- [ ] **T5 — API en contenedor** (`backend/Dockerfile`, servicio `api`, `make api`)
  - Depende de: T3, T4
  - RF: RF-2, RF-16
  - Hecho cuando: con la BD sana, `make api` deja `curl localhost:8080/health/ready` en 200; con la BD parada, el contenedor `api` queda en `exited (1)` con el error en sus logs.

- [ ] **T6 — Esqueleto Vue** (Vite, TS strict, Tailwind v4, shadcn-vue, Router, Pinia, Query, vue-i18n, Vitest, `es.json`, test de claves)
  - Depende de: T0
  - RF: RF-28, RF-29, RF-30
  - Hecho cuando: `npm run build` sin errores de tipos, `make test-web` pasa, y añadir un `t('clave.inexistente')` hace fallar el test indicando esa clave.

- [ ] **T7 — Pantalla de estado** (`fetchReadiness`, `useReadiness`, `HealthStatus`, `StatusPage`)
  - Depende de: T3 (contrato HTTP), T6
  - RF: RF-19, RF-20, RF-21, RF-22, RF-23, RF-24, RF-25, RF-26, RF-27
  - Hecho cuando: `make test-web` pasa con tests de 200, 503 `database_unavailable`, timeout de 5 s y error de red, alerta visible en fallo, y nueva consulta a los 30 s con timers falsos.

- [ ] **T8 — Web en contenedor y arranque unificado** (`frontend/Dockerfile`, servicio `web`, `make web`, `make up`)
  - Depende de: T5, T7
  - RF: RF-3, RF-4, RF-5, RF-6
  - Hecho cuando: desde cero, `make up` deja los tres servicios en marcha y `localhost:5173` muestra API y BD disponibles; si la BD no está sana en 60 s, `make up` imprime el error en español y sale con 1 sin arrancar la API.

- [ ] **T9 — Documentación y demo**
  - Depende de: T1–T8
  - RF: todos (verificación)
  - Hecho cuando: `AGENTS.md` refleja los comandos y Tailwind v4, `make test` pasa completo, y la demo de la spec (pasos 0–5) se completa desde un clon limpio.
