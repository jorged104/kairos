# Tareas 003 — Usuarios, roles, permisos y equipos

Derivadas de [plan.md](plan.md) para [spec.md](spec.md), en orden de dependencia.
Un commit por tarea en `feat/003-usuarios`; ninguna se marca sin su verificación en verde.
Las tareas marcadas **(tú)** necesitan acceso al servidor o una comprobación manual tuya.

- [ ] **T1 — Configuración y base de tests** (`config`, `.env.example`, `db/store.go`, `db/migrations/embed.go`, `db/dbtest`, `Makefile`, servicio PostgreSQL en el job `test` de `preview.yml`)
  - Depende de: —
  - RF: RF-1, RF-2, RF-3
  - Hecho cuando: `config.Load` rechaza `AUTH_MODE` ausente o inválido y nombra la variable OIDC que falta (tests); `main` sale con 1 en esos casos; `dbtest.New` crea y borra una BD con las migraciones aplicadas; `actionlint` sin avisos.

- [ ] **T2 — Esquema y queries** (`00002_usuarios.sql`, `queries/*.sql`, `make sqlc`)
  - Depende de: T1
  - RF: soporte de RF-10, RF-20, RF-26, RF-71, RF-77, RF-101, RF-119
  - Hecho cuando: la migración sube y baja en limpio; sqlc genera sin errores; un test comprueba que `UPDATE`/`DELETE` sobre `audit_log` fallan y que dos identidades con el mismo emisor + `sub` no pueden coexistir.

- [ ] **T3 — Permisos y Principal** (`internal/authz`, loader con caché en `internal/auth`)
  - Depende de: T2
  - RF: RF-43, RF-44, RF-57, RF-58, RF-59, RF-60, RF-63, RF-64
  - Hecho cuando: tests unitarios de la matriz rol → permisos; `Can` resuelve ámbito de equipo con subequipos y rechaza objetivos `lead`/`admin`; la caché expira a los 60 s y se invalida por usuario.

- [ ] **T4 — Sesión, CSRF y `/api/me`** (`internal/auth`: session, middleware, csrf; router)
  - Depende de: T3
  - RF: RF-40, RF-41, RF-42, RF-45, RF-50, RF-123
  - Hecho cuando: tests de cookie (atributos, 8 h), 401 con cookie ausente/caducada/manipulada, 403 `account_inactive`, 403 `csrf_rejected` con `Origin` ajeno, `/api/me` con roles, equipos y permisos; `/health/*` sigue sin sesión.

- [ ] **T5 — Auditoría** (`internal/audit`)
  - Depende de: T2
  - RF: RF-117, RF-118, RF-119, RF-120
  - Hecho cuando: el `Recorder` escribe en la transacción del cambio (si la tx se deshace, no queda entrada); `GET /api/audit` pagina y filtra por usuario, actor y fechas.

- [ ] **T6 — Login, registro, logout local y bootstrap** (`internal/auth`)
  - Depende de: T4, T5
  - RF: RF-4, RF-6, RF-7 a RF-14, RF-46, RF-51, RF-53, RF-54, RF-55, RF-56, RF-80, RF-82
  - Hecho cuando: tests de integración de cada RF, incluido el bloqueo al 5.º fallo por email y por IP aunque la contraseña sea correcta, y el mismo código para email inexistente y contraseña mala.

- [ ] **T7 — Equipos** (`internal/teams`)
  - Depende de: T4, T5
  - RF: RF-88, RF-89, RF-90, RF-90a, RF-90b, RF-90c, RF-90d, RF-91, RF-92
  - Hecho cuando: tests de alta, duplicado, ciclo, padre archivado, archivo con hijos, líderes con rol `lead` y ámbito heredado.

- [ ] **T8 — Atributos dinámicos** (`internal/attributes`)
  - Depende de: T4, T5
  - RF: RF-94 a RF-107
  - Hecho cuando: tests de cada tipo, opciones obligatorias en selección, cambio de tipo bloqueado con valores, retirada y restauración de atributos y opciones, duplicado entre activos.

- [ ] **T9 — Gestión de usuarios** (`internal/users`: alta, edición, estados, contraseñas, roles y equipos)
  - Depende de: T6, T7, T8
  - RF: RF-61, RF-63, RF-64, RF-65, RF-66 a RF-79, RF-78a, RF-81, RF-83 a RF-87, RF-98, RF-99, RF-108
  - Hecho cuando: tests de integración de cada RF, incluidos último administrador, baja reversible a desactivado, `idp_managed` y conservación de datos al desactivar.

- [ ] **T10 — Directorio** (`internal/users`: listado con Squirrel)
  - Depende de: T9
  - RF: RF-109 a RF-116
  - Hecho cuando: tests de paginación (25/100), búsqueda sin acentos ni mayúsculas, filtros combinados, equipo con subequipos, rangos de número/fecha, exclusión de dados de baja, campos y filtros visibles por rol, `invalid_filter`; con 500 usuarios y 50 atributos responde en < 500 ms en local.

- [ ] **T11 — OIDC: login, JIT, vinculación y homologación** (`internal/auth/oidc*`, `internal/idpmappings`, IdP simulado)
  - Depende de: T9
  - RF: RF-5, RF-15 a RF-39, RF-22a, RF-52, RF-53, RF-70
  - Hecho cuando: tests contra el IdP simulado de cada RF, incluido `state` reutilizado/caducado sin llamar al IdP, IdP que tarda > 5 s, token con firma/`aud`/`nonce` malos, comodín `/*`, manual vs IdP, `OIDC_ROLE_SOURCE=local`, último admin y alta concurrente de la misma identidad.

- [ ] **T12 — Tiempo real** (`internal/realtime`, `/ws`)
  - Depende de: T4
  - RF: RF-47, RF-48, RF-49
  - Hecho cuando: test que conecta con cookie válida, se rechaza sin ella y ve la conexión cerrada tras desactivar al usuario (intervalos reducidos en el test).

- [ ] **T13 — Test de la matriz de permisos completa**
  - Depende de: T9, T10, T11
  - RF: RF-62
  - Hecho cuando: un test de tabla recorre todas las operaciones protegidas de la API con `admin`, `lead` en y fuera de ámbito, `member` y `viewer`, y obtiene el código que indica la matriz.

- [ ] **T14 — Keycloak de desarrollo** (`docker-compose.yml` con `profile: sso`, `scripts/keycloak/realm-export.json`, `make up-sso`, `.env.example`)
  - Depende de: T11
  - RF: RF-126, RF-127
  - Hecho cuando: `make up-sso` levanta Keycloak con el realm importado y el login SSO de un usuario de ejemplo funciona contra la API; `make up` no levanta Keycloak.

- [ ] **T15 — Frontend: base de autenticación** (cliente API, store, guardia, layout, login, registro, proxy `/api` y `/ws`)
  - Depende de: T6
  - RF: RF-6, RF-12, RF-121, RF-122, RF-124, RF-125
  - Hecho cuando: tests de Vitest de la guardia (redirige y vuelve), de la traducción de errores por código y del catálogo; `npm run build` en verde.

- [ ] **T16 — Frontend: directorio y ficha de usuario** (`/users`, `/users/new`, `/users/:id`, `/account`)
  - Depende de: T15, T10
  - RF: RF-86, RF-109 a RF-115 (UI), RF-80
  - Hecho cuando: build y tests en verde; desde el navegador se busca, filtra, crea, edita, desactiva y se cambia la contraseña propia.

- [ ] **T17 — Frontend: administración** (`/teams`, `/attributes`, `/idp-mappings`, `/audit`)
  - Depende de: T16
  - RF: UI de RF-88 a RF-107, RF-39, RF-120
  - Hecho cuando: build y tests en verde; desde el navegador se gestionan equipos (con subequipos), atributos, homologaciones y se consulta la auditoría.

- [ ] **T18 — Previsualización y documentación** (`deploy/preview/compose.yml`, `preview.env.example`, `README.md`, `frontend/nginx.conf`, `AGENTS.md`, `docs/adr/`)
  - Depende de: T17
  - RF: — (despliegue de lo anterior)
  - Hecho cuando: `docker build --target prod ./frontend` pasa; `docker compose config` del compose de previsualización valida con un env de prueba; README y `preview.env.example` listan las variables nuevas; `AGENTS.md` actualiza el árbol de specs; ADR de sesión BFF.

- [ ] **T19 — Variables en el servidor (tú)**
  - Depende de: T18, **antes** de fusionar en `main`
  - Hecho cuando: `~/kairos/preview.env` incluye `AUTH_MODE`, `SESSION_SECRET` y `PUBLIC_URL`.

- [ ] **T20 — Demo manual (tú, con apoyo)**
  - Depende de: T14, T17
  - RF: todos (verificación)
  - Hecho cuando: se completan los pasos 1 a 19 de los criterios de finalización de la spec.
