# Tareas 002 — Entorno de previsualización

Derivadas de [plan.md](plan.md) para [spec.md](spec.md), en orden de dependencia.
Un commit por tarea en `feat/002-entorno-previsualizacion`; ninguna se marca sin su verificación en verde.
Las tareas marcadas **(tú)** necesitan acceso al servidor o a la configuración de GitHub/Cloudflare.

- [ ] **T1 — Imágenes de producción** (target `migrate` en `backend/Dockerfile`; `frontend/Dockerfile` multi-stage `dev`/`prod`; `frontend/nginx.conf`; `docker-compose.yml` local usa `target: dev`)
  - Depende de: —
  - RF: RF-5, RF-16
  - Hecho cuando: `docker build --target migrate ./backend` y `docker build --target prod ./frontend` terminan bien; la imagen `prod` sirve `/`, devuelve `index.html` en una ruta inventada y reenvía `/health/ready` a la API con respuesta 200; `make up` en local sigue funcionando igual.

- [ ] **T2 — Compose de previsualización** (`deploy/preview/compose.yml`, `deploy/preview/preview.env.example`)
  - Depende de: T1
  - RF: RF-12, RF-13, RF-16, RF-19, RF-20, RF-21
  - Hecho cuando: con un `preview.env` de prueba y una red Docker `traefik-test`, `docker compose -p kairos-preview up` levanta los cuatro servicios; ningún contenedor publica puertos en el host; solo `web` está en `traefik-test`; tras `docker compose restart`, los datos de la BD siguen ahí.

- [ ] **T3 — Script de despliegue** (`deploy/preview/deploy.sh`)
  - Depende de: T2
  - RF: RF-5, RF-8, RF-9, RF-10, RF-11, RF-17
  - Hecho cuando:
    - `shellcheck` (en contenedor) no da avisos.
    - Simulado en local con un Traefik temporal en `traefik-test`:
      - un despliegue bueno responde 200 a través de Traefik y deja `last-good` actualizado;
      - una migración rota termina con 1 y deja la versión anterior respondiendo;
      - una API que no arranca (en una rama de usar y tirar) vuelve a la versión anterior en ≤ 60 s y termina con 1.
    - En la salida del script no aparecen el hostname ni la contraseña.

- [ ] **T4 — Workflow de GitHub Actions** (`.github/workflows/preview.yml`)
  - Depende de: T3
  - RF: RF-1, RF-2, RF-3, RF-4, RF-6, RF-7, RF-14, RF-15
  - Hecho cuando: `actionlint` (en contenedor) no da avisos; el workflow solo se dispara con `push` a `main`; el job `deploy` depende de `test`, usa `runs-on: [self-hosted, kairos-preview]` y `concurrency` sin cancelar; no hay ningún disparador `pull_request`.

- [ ] **T5 — Documentación** (`deploy/preview/README.md`, `AGENTS.md`)
  - Depende de: T4
  - RF: — (soporte de RF-7, RF-13, RF-17 y del requisito de compatibilidad de migraciones)
  - Hecho cuando: el README explica, paso a paso, cómo instalar el runner como servicio con usuario dedicado, crear `~/kairos/preview.env`, averiguar la red y el entrypoint de Traefik, añadir el hostname en Cloudflare y activar la aprobación de workflows de forks; `AGENTS.md` incluye la sección de previsualización y la regla de migraciones compatibles.

- [ ] **T6 — Puesta en marcha del servidor (tú)**
  - Depende de: T5 y PR fusionado en `main`
  - RF: RF-7, RF-13, RF-20
  - Hecho cuando: el runner `kairos-preview` aparece como *Idle* en GitHub y sobrevive a un reinicio; `~/kairos/preview.env` existe con permisos `600`; el hostname resuelve a través del túnel; la aprobación de workflows de forks está en «all external contributors».

- [ ] **T7 — Verificación en el servidor**
  - Depende de: T6
  - RF: todos (verificación)
  - Hecho cuando se cumplen los criterios de finalización de la spec:
    - un push a `main` aparece en la URL con API y BD disponibles;
    - un push con un test en rojo queda en ✗ y la URL no cambia;
    - una API que no arranca vuelve a la anterior en ≤ 60 s;
    - tras reiniciar el servidor, la URL vuelve sola con los datos;
    - Home Assistant sigue accesible;
    - el hostname no aparece en el repo ni en los logs de Actions.
