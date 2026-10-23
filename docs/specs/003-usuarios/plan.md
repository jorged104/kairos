# Plan 003 — Usuarios, roles, permisos y equipos

Plan de implementación de [spec.md](spec.md) (aprobada 2026-10-22).
Aquí van las decisiones técnicas; el comportamiento lo fija la spec. Las «Decisiones de diseño
acordadas» de la spec (modo de autenticación, BFF, JIT, sesión propia, homologación, roles,
bootstrap, Keycloak en `profile: sso`) son restricciones de entrada y no se repiten.

## Punto de partida

- **Spec 001:** API Go (Chi, pgx, sqlc) con `/health/live` y `/health/ready`, frontend Vue con
  una única pantalla de estado, i18n con `es.json` y test que exige que toda clave usada exista.
- **Spec 002:** cada push a `main` se despliega en el servidor de previsualización. Como la API
  no arrancará sin `AUTH_MODE` (RF-2), el compose de previsualización y su `preview.env` tienen
  que recibir la configuración nueva **antes** de fusionar.
- No hay aún tablas de dominio, autenticación, WebSocket ni componentes de formulario en el
  frontend.

## Dependencias nuevas

| Paquete | Uso |
|---|---|
| `github.com/coreos/go-oidc/v3` + `golang.org/x/oauth2` | Descubrimiento, Authorization Code + PKCE y validación del `id_token` con la JWKS. Genérico, sin nada de Keycloak. |
| `github.com/golang-jwt/jwt/v5` | Cookie de sesión (HS256). Ya está en el grafo como indirecta. |
| `golang.org/x/crypto/argon2` | Hash de contraseñas argon2id con sal y coste configurable. bcrypt se descarta porque trunca a 72 bytes y la spec admite 128 caracteres. |
| `github.com/Masterminds/squirrel` | Solo para la consulta del directorio (filtros dinámicos). |
| `github.com/go-playground/validator/v10` | Validación de los cuerpos de petición. |
| `github.com/gorilla/websocket` | Endpoint de tiempo real autenticado (RF-47 a RF-49). |
| `github.com/google/uuid` | Identificadores en Go. Ya indirecta. |
| `github.com/pressly/goose/v3` (como librería) | Aplicar migraciones en los tests de integración. Ya está como tool. |
| Frontend: componentes shadcn-vue `button`, `input`, `label`, `table`, `dialog`, `select`, `checkbox`, `tabs`, `dropdown-menu`, `separator` | Pantallas de login y administración. Sin dependencias npm nuevas salvo las que arrastre shadcn-vue (Reka UI ya está). |

## Configuración (variables de entorno)

| Variable | Obligatoria | Por defecto | Notas |
|---|---|---|---|
| `AUTH_MODE` | sí | — | `local`, `oidc` o `both` (RF-1, RF-2). |
| `SESSION_SECRET` | sí | — | ≥ 32 bytes. Firma la cookie de sesión y la del flujo OIDC. |
| `PUBLIC_URL` | sí | — | Origen del frontend (`http://localhost:5173`). Se usa para CSRF, `CheckOrigin` del WebSocket y las redirecciones. |
| `COOKIE_SECURE` | no | `true` | `false` solo en `.env.example` de desarrollo. |
| `REGISTRATION_ENABLED` | no | `false` | RF-11, RF-12. |
| `DEFAULT_ROLE` | no | `member` | Vacío = rechazar el login sin rol (RF-35, RF-36). Debe ser uno de los cuatro roles. |
| `BOOTSTRAP_ADMIN_EMAIL` | no | — | RF-54 a RF-56. |
| `PASSWORD_HASH_MEMORY_KIB` / `PASSWORD_HASH_TIME` | no | `65536` / `3` | Coste de argon2id. |
| `TRUST_PROXY` | no | `false` | Si es `true`, la IP del cliente sale de `X-Real-IP` (la fija nginx); si no, de `RemoteAddr`. |
| `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` | si `AUTH_MODE` ≠ `local` | — | RF-3: si falta alguna, la API no arranca y nombra la que falta. |
| `OIDC_DISCOVERY_URL` | no | `OIDC_ISSUER` | Para cuando la API ve al IdP por otra URL (contenedor `keycloak:8080`) que el navegador (`localhost:8180`). Usa `oidc.InsecureIssuerURLContext`, que sigue validando el `iss` del token contra `OIDC_ISSUER`. |
| `OIDC_ROLE_SOURCE` | no | `idp` | `idp` o `local` (RF-33). |
| `OIDC_LINK_BY_EMAIL` | no | `false` | Vinculación por email verificado (RF-24). |

`config.Load` valida todo al arrancar y devuelve un error con el nombre de la variable; `main`
lo registra y sale con 1 (RF-2, RF-3). El IdP **no** se contacta al arrancar: el descubrimiento
se hace en el primer login y se cachea, para que un IdP caído no tumbe la API (RF-19).

## Esquema (migración `00002_usuarios.sql`, solo aditiva)

```
users(id uuid pk default gen_random_uuid(), name text, email text,
      email_normalized text unique,            -- lower(btrim(email)), RF-10, RF-71
      status text check (active|inactive|offboarded), created_at, updated_at)
user_identities(id uuid pk, user_id fk, issuer text, subject text, created_at,
      unique(issuer, subject))                 -- RF-20, RF-26 (la unique resuelve la carrera)
local_credentials(user_id pk fk, password_hash text, updated_at)
user_roles(user_id fk, role text check (admin|lead|member|viewer),
      source text check (manual|idp), created_at, primary key(user_id, role))
teams(id uuid pk, name text, name_normalized text unique, parent_id uuid null fk teams,
      external_ref text null, archived_at timestamptz null, created_at, updated_at)
team_members(team_id fk, user_id fk, source text check (manual|idp), created_at,
      primary key(team_id, user_id))
team_leaders(team_id fk, user_id fk, created_at, primary key(team_id, user_id))
attributes(id uuid pk, name text, name_normalized text, type text check
      (text|number|date|boolean|select), required bool, options jsonb default '[]',
      retired_at timestamptz null, created_at, updated_at)
      unique index (name_normalized) where retired_at is null   -- RF-101
user_attribute_values(user_id fk, attribute_id fk, value jsonb, updated_at,
      primary key(user_id, attribute_id))
idp_mappings(id uuid pk, issuer text, claim text check (roles|groups), external_value text,
      target_type text check (role|team), target text, created_at,
      unique(issuer, claim, external_value, target_type, target))
audit_log(id bigint identity pk, occurred_at timestamptz default now(), actor_id uuid null,
      actor_kind text check (user|idp|system), action text, target_type text,
      target_id text, before jsonb, after jsonb)
oidc_login_states(state text pk, nonce text, code_verifier text, return_to text, created_at)
create extension if not exists unaccent;      -- RF-110
```

- **Clave `(user_id, role)` con `source`:** un rol aparece una sola vez por usuario. Si el IdP
  calcula un rol que ya es manual, no se toca (RF-32). Igual en `team_members`.
- **Opciones de selección** en `attributes.options` como `[{"value": "...", "retired": false}]`.
  Retirar una opción la marca, no la borra (RF-107).
- **Valores** en `jsonb` escalar (texto, número, `YYYY-MM-DD`, booleano, valor de opción). El
  directorio compara con `value #>> '{}'` y castea a `numeric`/`date` para los rangos.
- **Auditoría inmutable (RF-119):** trigger `BEFORE UPDATE OR DELETE` que lanza excepción.
- **Jerarquía de equipos:** descendientes y antecesores con `WITH RECURSIVE`. A 500 usuarios no
  merece la pena una tabla de cierre.
- `oidc_login_states` se borra al consumirse (`DELETE … RETURNING`) y las filas de más de 10 min
  se purgan en cada inicio de login (RF-17).

## Backend

### Paquetes

```
internal/
  config/        + AuthConfig, OIDCConfig, validación
  db/
    migrations/  00002_usuarios.sql + embed.go (//go:embed, para los tests)
    queries/     users.sql, identities.sql, roles.sql, teams.sql, attributes.sql,
                 idp_mappings.sql, audit.sql, oidc_states.sql
    store.go     Store{pool, *sqlc.Queries} con InTx(ctx, func(*sqlc.Queries) error)
    dbtest/      crea una BD temporal por test a partir de TEST_DATABASE_URL y aplica migraciones
  authz/         Role, Permission, matriz única rol→permisos (RF-59), Principal, Scope
  auth/          password (argon2id), session (JWT cookie), middleware, CSRF, rate limit,
                 principal loader con caché, handlers de login/registro/logout/me,
                 oidc (BFF), sync de homologaciones, bootstrap
  users/         alta, edición, estados, contraseñas, roles y equipos de un usuario, directorio
  teams/         CRUD de equipos, jerarquía, líderes
  attributes/    definición de atributos y validación de valores
  idpmappings/   CRUD de homologaciones
  audit/         Recorder (escribe en la tx del cambio) + consulta paginada
  realtime/      hub WebSocket autenticado
```

Cada dominio mantiene `handler.go` / `service.go` / `repository.go` / `types.go`. Los servicios
reciben el `Principal` del contexto y deciden con permisos; los handlers solo traducen HTTP.

### Permisos (RF-57 a RF-65)

`authz` define permisos con ámbito, nunca roles en los endpoints:

```
directory:read           (todos)
users:read_full:all | users:read_full:team | (propio implícito)
users:create:all | users:create:team
users:update:all | users:update:team
users:status:all | users:status:team       desactivar / reactivar
users:offboard                             dar de baja y revertir
roles:assign:all | roles:assign:basic      basic = member, viewer
teams:members:all | teams:members:team
teams:manage, attributes:manage, idp_mappings:manage, passwords:set, audit:read
self:password
```

`authz.Can(principal, perm, target)` resuelve el ámbito: `:all` gana; `:team` exige que el
objetivo pertenezca (directa o indirectamente, RF-90d) a un equipo que el actor lidera y que su
rol de mayor prioridad sea `member` o `viewer` (RF-63). El `Principal` lleva `UserID`, `Status`,
`Roles`, `Permissions` y `ScopeTeamIDs` (equipos liderados + descendientes, no archivados).

**Caché (RF-43, RF-44):** el middleware carga el `Principal` de BD con una caché en memoria de
60 s por usuario. Toda mutación sobre un usuario en esta instancia invalida su entrada; la TTL
cubre el caso general (máximo 60 s < 120 s).

### Sesión, CSRF y rate limit

- **Cookie `kairos_session`:** JWT HS256 con `sub` y `exp` (8 h, sin renovación, RF-41).
  `HttpOnly`, `SameSite=Lax`, `Secure` según `COOKIE_SECURE`, `Path=/`.
- **Cookie `kairos_auth_method`** (`local`/`oidc`, httpOnly, misma caducidad): solo sirve para
  decidir en el logout si hay que ir al IdP (RF-52). No forma parte de la identidad.
- **Middleware `RequireSession`:** 401 `unauthenticated` sin cookie, firma mala o caducada
  (RF-42); 403 `account_inactive` si el usuario no está `active` (RF-45).
- **CSRF:** en métodos que modifican, `Origin` (o `Referer` si falta) debe coincidir con
  `PUBLIC_URL`; si no, 403 `csrf_rejected`. El callback OIDC es `GET` y se protege con `state`.
- **Rate limit (RF-9):** en memoria, ventana deslizante de 15 min por email normalizado y por IP;
  al quinto fallo se bloquea 15 min. Una sola instancia de API: suficiente para esta iteración.
- **Contraseñas:** 12 a 128 caracteres (contados en runas). Formato PHC
  `$argon2id$v=19$m=…,t=…,p=1$sal$hash`, comparación en tiempo constante. Si el email no existe
  se calcula igualmente un hash de relleno para no revelar su existencia por tiempo (RF-8).

### OIDC (RF-15 a RF-39)

1. `GET /api/auth/oidc/start?return_to=/ruta` → genera `state`, `nonce` y `code_verifier`, los
   guarda en `oidc_login_states`, pone `state` en una cookie corta `kairos_oidc_state` y
   redirige al IdP (`S256`).
2. `GET /api/auth/oidc/callback` → comprueba que `state` coincide con la cookie y existe en BD
   con menos de 10 min **antes** de llamar al IdP (RF-17); intercambia el código con un cliente
   HTTP de 5 s (RF-19 → `idp_unavailable`); valida el `id_token` con `go-oidc` (firma, `iss`,
   `aud`, `exp`) y `nonce` (RF-16, RF-18: el log incluye el motivo, nunca el token).
3. En una transacción: resolver identidad (RF-20 a RF-27) → actualizar nombre/email (RF-22,
   RF-22a) → bootstrap (RF-54, RF-55) → sincronizar homologaciones (RF-28 a RF-38, con
   auditoría `actor_kind=idp`) → comprobar estado (RF-46).
4. Abre sesión y redirige a `return_to` (solo rutas relativas que empiezan por `/`). Los errores
   redirigen a `/login?error=<code>` para que el frontend los traduzca (RF-125).

Homologación de grupos: un `external_value` que termina en `/*` casa con `valor/…` a cualquier
profundidad, nunca con el propio padre (RF-29). Los equipos archivados se ignoran con aviso en
log (RF-93). Si el claim `roles` o `groups` falta, se trata como lista vacía.

Logout: `POST /api/auth/logout` borra las cookies y responde `{ "redirect_url": … }`: el
`end_session_endpoint` con `client_id` y `post_logout_redirect_uri=PUBLIC_URL/login` si el login
fue OIDC y el IdP lo publica (RF-52, RF-53); si no, `/login`.

### API

Prefijo `/api`. JSON `snake_case`. Errores con el formato común; las validaciones añaden un
campo opcional `details` (`{ "error": { "code", "message", "details": { "field": "…" } } }`)
para indicar el atributo o campo inválido (RF-14, RF-97, RF-98).

| Método y ruta | Permiso | RF |
|---|---|---|
| `GET /api/auth/config` | público | RF-6, RF-12 |
| `POST /api/auth/login` · `POST /api/auth/register` | público | RF-4, RF-7 a RF-14 |
| `GET /api/auth/oidc/start` · `GET /api/auth/oidc/callback` | público | RF-5, RF-15 a RF-39 |
| `POST /api/auth/logout` | sesión | RF-51 a RF-53 |
| `GET /api/me` · `PUT /api/me/password` | sesión | RF-50, RF-80, RF-82 |
| `GET /api/users` | `directory:read` | RF-109 a RF-116 |
| `POST /api/users` · `GET/PATCH /api/users/{id}` | `users:*` | RF-66 a RF-73 |
| `POST /api/users/{id}/deactivate` · `/reactivate` · `/offboard` · `/restore` | `users:status:*`, `users:offboard` | RF-74 a RF-79 |
| `PUT /api/users/{id}/password` | `passwords:set` | RF-81 |
| `POST /api/users/{id}/roles` · `DELETE /api/users/{id}/roles/{role}` | `roles:assign:*` | RF-64, RF-65, RF-83 a RF-85 |
| `POST /api/users/{id}/teams` · `DELETE /api/users/{id}/teams/{team_id}` | `teams:members:*` | RF-83 a RF-87 |
| `GET/POST /api/teams` · `PATCH /api/teams/{id}` · `POST /api/teams/{id}/archive` · `/unarchive` | `teams:manage` (GET: sesión) | RF-88 a RF-93 |
| `POST /api/teams/{id}/leaders` · `DELETE /api/teams/{id}/leaders/{user_id}` | `teams:manage` | RF-91 |
| `GET/POST /api/attributes` · `PATCH /api/attributes/{id}` · `/retire` · `/restore` | `attributes:manage` (GET: sesión) | RF-94 a RF-107 |
| `GET/POST /api/idp-mappings` · `PATCH/DELETE /api/idp-mappings/{id}` | `idp_mappings:manage` | RF-39 |
| `GET /api/audit` | `audit:read` | RF-120 |
| `GET /ws` | sesión (cookie) | RF-47 a RF-49 |

Códigos de error nuevos (el frontend los traduce): `unauthenticated`, `forbidden`,
`account_inactive`, `csrf_rejected`, `invalid_credentials`, `too_many_attempts`,
`local_auth_disabled`, `oidc_disabled`, `registration_disabled`, `password_too_short`,
`password_too_long`, `wrong_password`, `email_conflict`, `validation_failed`,
`invalid_state`, `idp_unavailable`, `idp_token_invalid`, `account_conflict`, `email_required`,
`no_role`, `last_admin`, `idp_managed`, `user_offboarded`, `team_name_conflict`, `team_cycle`,
`team_archived`, `team_has_children`, `attribute_name_conflict`, `attribute_type_locked`,
`attribute_invalid_value`, `attribute_required`, `attribute_retired`, `option_required`,
`invalid_filter`, `not_found`, `leader_not_lead`, `idp_mapping_conflict`.

**Último administrador (RF-38, RF-79):** las operaciones que pueden quitar un admin activo toman
`pg_advisory_xact_lock` y cuentan admins activos dentro de la misma transacción.

**Directorio (RF-109 a RF-116):** Squirrel construye la consulta sobre `users`, con
`unaccent(lower(...)) LIKE` para el texto y `EXISTS` con CTE recursiva para equipo + subequipos.
Para quien no tiene `users:read_full:all`, el email, los roles y los atributos solo cuentan (en
búsqueda, filtros y respuesta) para los usuarios de su ámbito y para sí mismo. Los filtros por
atributo retirado solo los admite un admin; el resto recibe `invalid_filter`.

**Auditoría:** `audit.Recorder` escribe dentro de la transacción del cambio, así un cambio sin
auditoría no se confirma. `actor_kind = idp` para la sincronización del login.

### Tiempo real (RF-47 a RF-49)

`/ws` acepta la conexión solo con sesión válida y usuario activo (si no, 401/403 antes del
upgrade). `CheckOrigin` exige `PUBLIC_URL`. El hub registra conexiones por usuario y cada 30 s
revisa el estado con el loader (caché de 60 s): un usuario no activo ve su conexión cerrada en
≤ 90 s. En esta spec el hub solo envía `ping`; los eventos de dominio llegan con otras specs.

## Frontend

- **Cliente API** `src/api/client.ts`: `fetch` con `credentials: 'same-origin'`, parsea el error
  común y lanza `ApiError(code, status, details)`. Las pantallas muestran `t('errors.<code>')`
  con `errors.unknown` como respaldo (RF-125).
- **Proxy de Vite y nginx:** `/api` y `/ws` (con upgrade) hacia la API, junto a `/health`.
- **Sesión:** store Pinia `auth` con `me` (TanStack Query `['me']`). Guardia global del router:
  toda ruta salvo `/login` y `/register` requiere sesión; sin ella redirige a
  `/login?redirect=<ruta>` (RF-121) y tras entrar vuelve a `redirect` (RF-122). Un 401 en
  cualquier consulta invalida `me` y lleva al login.
- **Rutas:**

  | Ruta | Contenido |
  |---|---|
  | `/login`, `/register` | Métodos según `/api/auth/config` (RF-6), errores de `?error=` |
  | `/` | Pantalla de estado (spec 001), ahora protegida |
  | `/users` | Directorio: búsqueda, filtros, paginación |
  | `/users/new`, `/users/:id` | Alta y ficha: datos, atributos, roles y equipos con origen, estado, contraseña |
  | `/teams` | Árbol de equipos, alta, padre, archivo, líderes |
  | `/attributes` | Definición, retirada y restauración de atributos y opciones |
  | `/idp-mappings`, `/audit` | Homologaciones y auditoría (admin) |
  | `/account` | Perfil propio y cambio de contraseña |

- **Layout** con navegación que muestra solo lo que `me.permissions` permite y botón de logout.
- **i18n:** todo texto en `es.json` bajo `auth.*`, `users.*`, `teams.*`, `attributes.*`,
  `idpMappings.*`, `audit.*`, `nav.*`, `errors.*` (RF-124). El test de claves existente lo cubre.

## Entorno de desarrollo con IdP (RF-126, RF-127)

- `docker-compose.yml`: servicio `keycloak` (`quay.io/keycloak/keycloak:26.4`, `start-dev
  --import-realm`) con `profiles: [sso]`, puerto `8180`, `KC_HOSTNAME=http://localhost:8180` y
  `KC_HOSTNAME_BACKCHANNEL_DYNAMIC=true` para que la API lo alcance por `keycloak:8080`.
- `scripts/keycloak/realm-export.json`: realm `kairos`, cliente confidencial `kairos` con PKCE,
  mappers de `roles` (roles del cliente) y `groups` (ruta completa) en el `id_token`, grupos
  `/Areas/{BI,Innovacion,Desarrollo,CienciaDatos}` con algún subgrupo, y usuarios de ejemplo.
- `make up-sso` arranca además el profile `sso`. Sin él, nada cambia (RF-127).

## Previsualización (spec 002)

- `deploy/preview/compose.yml`: la API recibe `AUTH_MODE`, `SESSION_SECRET`, `PUBLIC_URL`,
  `COOKIE_SECURE=true`, `TRUST_PROXY=true` y el resto de variables opcionales desde
  `preview.env` (con `:?` para las obligatorias).
- `frontend/nginx.conf`: `/api/` y `/ws` (upgrade) hacia la API, `X-Real-IP`.
- `deploy/preview/README.md` y `preview.env.example`: las variables nuevas. **Hay que añadirlas
  al `~/kairos/preview.env` del servidor antes de fusionar a `main`**, o el despliegue fallará y
  se quedará la versión anterior.

## Tests

- **Integración con PostgreSQL:** `dbtest.New(t)` crea una BD `kairos_test_<aleatorio>` a partir
  de `TEST_DATABASE_URL`, aplica las migraciones embebidas y la borra al terminar. Si la variable
  no está, los tests de integración se saltan (`t.Skip`) en local; en CI es obligatoria.
- **CI (`preview.yml`, job `test`):** servicio `postgres:17-alpine` y `TEST_DATABASE_URL`.
  Se revisa con `actionlint`.
- `make test-api` pasa `TEST_DATABASE_URL` apuntando a la BD del `docker compose`.
- **IdP simulado** (`auth/oidctest`): servidor `httptest` con discovery, JWKS (RSA generada en el
  test), token endpoint y `end_session_endpoint` opcional, que emite `id_token` configurables.
- **Matriz de permisos (RF-62):** test de tabla que, para cada operación protegida y cada rol
  (`admin`, `lead` dentro y fuera de ámbito, `member`, `viewer`), comprueba el código HTTP.
- **Frontend (Vitest):** guardia del router (RF-121, RF-122), traducción de errores por código
  (RF-125) y el test de claves i18n (RF-124).

## Riesgos y limitaciones

- **Rate limit y caché en memoria:** válidos con una sola instancia de API. Con varias, el límite
  sería por instancia; se migraría a BD.
- **Ventana de 120 s:** aceptada en la spec; con la invalidación local, en una sola instancia los
  cambios hechos por la API son inmediatos.
- **`KC_HOSTNAME_BACKCHANNEL_DYNAMIC`** solo para desarrollo; en producción la API y el navegador
  verán el IdP por la misma URL y `OIDC_DISCOVERY_URL` se deja vacía.

## Tareas
Ver [tasks.md](tasks.md).

## Verificación
- `make test` en verde (backend con PostgreSQL, frontend con Vitest) y `npm run build`.
- `actionlint` sobre el workflow modificado.
- Demo manual de los criterios de finalización de la spec, pasos 1 a 19.
