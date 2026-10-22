# Plan 002 — Entorno de previsualización

Plan de implementación de [spec.md](spec.md) (aprobada 2026-10-22).
Aquí van las decisiones técnicas; el comportamiento lo fija la spec.

## Punto de partida

- **Servidor:** Linux x86_64 con Docker. Ya ejecuta Home Assistant y Traefik.
  El tráfico entra por un túnel de Cloudflare, que lo entrega a Traefik, y Traefik enruta por
  hostname leyendo labels de Docker.
- **Repo:** `jorged104/kairos` es **público**, y eso condiciona dos decisiones:
  - Los logs de GitHub Actions también son públicos. El hostname y las credenciales no pueden
    pasar nunca por el workflow (RF-17).
  - Un runner self-hosted en un repo público puede ejecutar código de forks en tu servidor.
    Se mitiga con las medidas de la sección «Seguridad del runner».
- **Spec 001:** deja una API Go con `/health/ready`, un frontend Vue servido con Vite dev, goose
  como tool de Go y `docker-compose.yml` pensado para desarrollo local.

## Arquitectura

```
push a main ─► GitHub Actions
                 ├─ job test   (runner de GitHub)  go test · npm test · npm run build
                 └─ job deploy (runner self-hosted en el servidor, needs: test)
                        └─ deploy/preview/deploy.sh

Internet ─► Cloudflare (HTTPS) ─► túnel ─► Traefik ─► web (nginx :80) ─/health─► api ─► db
                                            red externa de Traefik  │  red interna kairos-preview
```

- **Proyecto Compose propio** (`-p kairos-preview`), con su red interna y su volumen. Solo
  `web` se une a la red de Traefik. Ningún servicio publica puertos en el host (RF-19 a RF-21).
- **HTTPS:** lo termina Cloudflare (RF-16). Del túnel a Traefik va por el entrypoint HTTP
  que ya tienes.
- **Configuración del servidor:** vive en un fichero de entorno fuera del repo,
  `~/kairos/preview.env`, que solo lee `deploy.sh` (RF-17).

  | Variable | Ejemplo |
  |---|---|
  | `PREVIEW_HOST` | *(tu hostname privado)* |
  | `TRAEFIK_NETWORK` | `traefik` |
  | `TRAEFIK_ENTRYPOINT` | `web` |
  | `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | valores fuertes, solo en el servidor |

## Componentes

### 1. Imágenes de producción
- **`backend/Dockerfile`:** se añade un target `migrate`. Compila goose desde la tool de
  `go.mod` (`go build github.com/pressly/goose/v3/cmd/goose`) y copia
  `internal/db/migrations`. El target por defecto sigue siendo la API.
- **`frontend/Dockerfile`:** pasa a ser multi-stage.
  - `dev`: lo de ahora (Vite), usado por `docker-compose.yml` local.
  - `build`: ejecuta `npm ci && npm run build`.
  - `prod`: `nginx:alpine` con el `dist/` y un `frontend/nginx.conf` que reenvía `/health`
    a `api:8080` y devuelve `index.html` para las rutas de la SPA.

### 2. `deploy/preview/compose.yml`
- **`db`:** `postgres:17-alpine`, volumen `pgdata` persistente (RF-12), `restart:
  unless-stopped` y healthcheck con `pg_isready`. Sin puertos.
- **`migrate`:** imagen `kairos-migrate:${TAG}`, en el perfil `deploy`. Se ejecuta con
  `compose run --rm`.
- **`api`:** imagen `kairos-api:${TAG}`, `restart: unless-stopped`, `DATABASE_URL`
  apuntando a `db`.
- **`web`:** imagen `kairos-web:${TAG}`, `restart: unless-stopped`, en las redes `internal`
  y `${TRAEFIK_NETWORK}` (externa). Labels:
  - `traefik.enable=true`
  - ``traefik.http.routers.kairos-preview.rule=Host(`${PREVIEW_HOST}`)``
  - `traefik.http.routers.kairos-preview.entrypoints=${TRAEFIK_ENTRYPOINT}`
  - `traefik.http.services.kairos-preview.loadbalancer.server.port=80`
  - `traefik.docker.network=${TRAEFIK_NETWORK}`

`restart: unless-stopped` y Docker arrancando al iniciar el sistema cubren el reinicio del
servidor: vuelven a levantarse los contenedores de la última versión buena (RF-13).

### 3. `deploy/preview/deploy.sh` (bash, `set -euo pipefail`, nunca `set -x`)
1. Lee `~/kairos/preview.env`. Si falta, se puede indicar otro con `KAIROS_PREVIEW_ENV`.
2. Toma como `TAG` el SHA corto del commit y como `PREV` la etiqueta guardada en
   `~/kairos/last-good`.
3. `TAG=$TAG compose build api web migrate`. Si falla, sale con 1 sin tocar nada (RF-5).
4. `compose up -d --wait db`.
5. `compose run --rm migrate`. Si falla, sale con 1 y la versión anterior sigue en marcha
   (RF-8, RF-9).
6. `TAG=$TAG compose up -d api web`.
7. Comprueba la disponibilidad hasta 60 s: `compose exec web wget -qO- http://localhost/health/ready`.
   Esto prueba el camino completo nginx → api → db.
   - **OK:** guarda `TAG` en `last-good` y borra las imágenes `kairos-*` salvo las dos últimas
     etiquetas.
   - **Falla:** `TAG=$PREV compose up -d api web` y sale con 1 (RF-10). En el primer
     despliegue no hay `PREV`: sale con 1 y no hay nada que restaurar (caso límite de la spec).
8. Solo imprime pasos y etiquetas. Nunca el hostname, credenciales ni `compose config` (RF-17).

### 4. `.github/workflows/preview.yml`
- **Disparo:** `on: push: branches: [main]` (RF-1, RF-2).
- **Cola:** `concurrency: { group: preview, cancel-in-progress: false }` (RF-6).
- **Job `test`** (`ubuntu-latest`, en GitHub): `setup-go` (versión de `go.mod`) y
  `setup-node@24`, luego `go test ./...`, `npm ci`, `npm test -- --run` y `npm run build`
  (RF-3, RF-4).
- **Job `deploy`** (`runs-on: [self-hosted, kairos-preview]`, `needs: test`): checkout y
  `bash deploy/preview/deploy.sh`.
  - El resultado aparece como ✓ o ✗ en el commit (RF-14) y el log de cada paso queda en
    Actions (RF-15).
  - Si el servidor está apagado, el job espera en cola a que el runner vuelva (RF-7).
- **Permisos:** `permissions: contents: read`.

### 5. Documentación
- **`deploy/preview/README.md`:** puesta en marcha del servidor, paso a paso. Cubre el runner,
  el fichero de entorno, la red y el entrypoint de Traefik, y el hostname en Cloudflare.
- **`AGENTS.md`:**
  - Nueva sección «Previsualización».
  - Nueva convención: cada migración debe dejar funcionando la versión anterior de la API
    (cambios aditivos; borrar o renombrar se hace en dos despliegues). Es el requisito no
    funcional de compatibilidad.

## Seguridad del runner (repo público)
- El workflow **solo** se dispara con `push` a `main`. No hay ningún `pull_request` que use el
  runner self-hosted.
- En GitHub, *Settings → Actions → General → Fork pull request workflows*: marca «Require
  approval for all external contributors». Lo tienes que hacer tú.
- El runner usa un usuario dedicado (`kairos-runner`) que solo pertenece al grupo `docker` y
  tiene la etiqueta `kairos-preview`.
- Riesgo que queda: el grupo `docker` equivale a root en el servidor. Si eso no es aceptable,
  la alternativa es hacer el repo privado.

## Limitaciones conocidas
- **Downtime breve (RF-11):** al sustituir los contenedores `api` y `web` hay unos segundos
  sin servicio. Evitarlo exigiría un despliegue blue/green con dos stacks, más complejo que lo
  que necesita una previsualización.
- **Cola de despliegues (RF-6):** GitHub mantiene en cola un solo despliegue pendiente. Si
  llegan tres pushes seguidos, el del medio se cancela y se despliega el más reciente.
- **Servidor apagado (RF-7):** GitHub descarta los jobs que llevan más de 24 h en cola. Si el
  servidor está apagado más tiempo, ese push no se despliega y hay que esperar al siguiente
  (o relanzar el workflow desde GitHub).

## Tareas
Ver [task.md](task.md).

## Verificación
- **En local** (Windows con Docker):
  - `shellcheck` sobre `deploy.sh`, ejecutado en contenedor.
  - Simulación de `deploy.sh` contra una red Docker de prueba y un Traefik temporal:
    despliegue OK, migración rota y API que no arranca (en una rama de usar y tirar).
- **En el servidor**, contigo: la lista de criterios de finalización de la spec.
