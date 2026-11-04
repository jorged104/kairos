# Plan 015 — Administración del sistema

Plan de implementación de [spec.md](spec.md), aprobada el 2026-11-04. Aquí van las decisiones técnicas; el comportamiento lo fija la spec.

## Dependencias y esquema
No hay dependencias nuevas:
- el cifrado usa `crypto/aes` y `crypto/cipher` de la biblioteca estándar;
- el S3 usa el cliente minio que ya existe.

Migración `00013_sistema.sql`, solo aditiva:
- **`system_settings`**: una sola fila, con `id boolean PRIMARY KEY CHECK (id)`.
  - Columnas:
    - `version bigint`;
    - `auth_mode`, `registration_enabled` y `default_role`;
    - `oidc_issuer`, `oidc_discovery_url`, `oidc_client_id` y `oidc_client_secret bytea`, este cifrado;
    - `oidc_test jsonb`, con `{ok, email, name, at, fingerprint}`;
    - `files_kind` (`db` o `s3`), `s3_endpoint`, `s3_bucket`, `s3_region`, `s3_access_key`, `s3_secret_key bytea` (cifrado) y `s3_use_ssl`;
    - `theme_palette`, `theme_primary` y `theme_radius`;
    - `updated_at`.
  - La migración crea la tabla vacía. La fila la crea la API en el primer arranque, a partir de las variables de entorno (RF-8), porque la migración no las conoce.
- **`user_preferences`**: `user_id` como clave primaria, y `theme_mode`, con `light`, `dark` o `system` (RF-48 a RF-50).
- **`oidc_login_states`**: añade `purpose text NOT NULL DEFAULT 'login'`, con `login` o `test`. La versión anterior de la API sigue insertando estados sin indicarlo y se leen como `login`.

## Backend
### `internal/settings` (nuevo)
- **`Open(ctx, store, cfg)`:**
  - si no hay fila, la inserta con lo que haya en `cfg` (RF-8);
  - devuelve un `*Holder` con una foto de la configuración en un `atomic.Pointer`.
- **`Holder`:**
  - `Auth()`, `OIDC()` y `Files()` devuelven los `config.*Config` de siempre, mezclando el entorno con la fila (RF-9, RF-10);
  - la URL de retorno se calcula como `PublicURL + /api/auth/oidc/callback` (RF-25);
  - `Theme()` devuelve la apariencia, y `Version()` la versión.
- **`Holder.Run(ctx)`:** relee la versión cada 5 s y recarga si cambió, para el caso de varias instancias. En la instancia que guarda, el cambio se ve al momento (RF-3).
- **Cifrado:**
  - AES-256-GCM con una clave derivada de `SESSION_SECRET`: `sha256("kairos/settings/v1" + secret)`.
  - Si `SESSION_SECRET` cambia, los secretos guardados no se pueden descifrar. Se tratan como no configurados, se escribe un aviso en el log y la pantalla pide escribirlos de nuevo.
- **Servicio y handler:** `GET /api/system/settings` y `PUT /api/system/settings/{access|sso|storage|appearance}`, con el permiso nuevo `system:manage`, solo para admin (RF-1, RF-2).
  - Las respuestas no llevan secretos, solo `client_secret_set` y `secret_key_set` (RF-6). Un secreto vacío conserva el anterior (RF-7).
  - **access:** valida RF-11 a RF-16.
    - `sso_not_configured` cuando el SSO no está completo;
    - `sso_not_tested` cuando no hay un login de prueba correcto con la huella actual del SSO;
    - `no_emergency_access` cuando no hay acceso de emergencia (RF-23), salvo que la petición lleve `accept_no_emergency: true`.
  - **sso:**
    - `sso_in_use` si el modo es «solo SSO» (RF-32);
    - comprueba el descubrimiento con un límite de 5 s, y si falla responde `sso_discovery_failed`, con el detalle `reason` (RF-26, RF-27);
    - borra `oidc_test` si cambió la huella (RF-31).
  - **storage:** si hay archivos, responde `storage_locked`, con el detalle `files` (RF-37, RF-38). La comprobación de escribir, leer y borrar falla con `s3_check_failed` y el detalle `step` (RF-35, RF-36). La comprobación es una función que los tests pueden sustituir.
  - **appearance:** la paleta, el color primario y el radio se validan contra las listas (RF-39, RF-40, RF-43).
  - Cada cambio sube `version` y se audita en la misma transacción como `system.settings_changed`. En los secretos solo se registra `"changed"` (RF-4).
- **`GET /api/system/settings`** devuelve también:
  - `emergency_available`: hay administrador inicial y tiene contraseña local;
  - `files_count`;
  - `redirect_url`;
  - el resultado del último login de prueba.

### Cambios en lo existente
- **`auth.Service`, `auth.Handler`, `auth.OIDC`, `users.Service` e `idpmappings`** reciben funciones `func() config.AuthConfig` / `config.OIDCConfig` del `Holder`, en lugar de valores fijos.
- **`auth.OIDC`** guarda el proveedor descubierto junto con la huella de la configuración, y lo descubre de nuevo si cambió.
- **Login de prueba:**
  - `GET /api/auth/oidc/start?test=1` exige una sesión con `system:manage` y guarda el estado con `purpose='test'`;
  - el callback de un estado `test` valida el `id_token`, guarda el resultado en `oidc_test` y vuelve a `/admin/system?tab=sso`;
  - no abre sesión, no crea usuarios y no los vincula (RF-28 a RF-31).
- **Acceso de emergencia:**
  - en modo «solo SSO», `Login` acepta solo el email del administrador inicial (RF-19), con el mismo límite de intentos (RF-21);
  - si entra, audita `auth.emergency_login` (RF-22);
  - `/api/auth/config` añade `emergency_enabled` y `theme` (RF-20, RF-45).
- **`files.Storage`** construye el cliente S3 desde `Holder.Files()` y lo cambia cuando cambia la configuración. El arranque ya no comprueba el bucket: lo comprueba el guardado (RF-35).
- **`/api/me`** añade `theme_mode`, y se añade `PUT /api/me/preferences` (RF-48 a RF-50).
- **`server.NewApp`** recibe el `*settings.Holder`. `main.go` y `apitest.New` llaman a `settings.Open`.

### Tests
- apitest de:
  - la primera copia desde las variables de entorno;
  - los secretos que no salen;
  - el cambio de modo y sus rechazos;
  - el login de prueba con el IdP simulado: que no abre sesión y que un cambio del SSO lo invalida;
  - el acceso de emergencia y su auditoría;
  - el S3 con la comprobación simulada y el bloqueo con archivos;
  - la apariencia;
  - las preferencias;
  - la fila en la matriz.
- Test unitario del cifrado de ida y vuelta.

## Frontend
- **`lib/theme.ts`:**
  - contiene los datos de las paletas (los valores oklch de shadcn) y de los colores primarios (claro y oscuro, cada uno con su texto);
  - `themeCss(theme)` genera `:root { … } .dark { … }`;
  - `applyAppearance(theme, mode)` escribe un `<style id="kairos-theme">` y pone la clase `dark` según el modo y `prefers-color-scheme`.
  - La paleta Neutral con primario por defecto y radio suave reproduce exactamente `style.css` (RF-46).
- **`composables/useAppearance.ts`:** se usa en `App.vue`. Aplica el tema de `/api/auth/config` y el `theme_mode` de `auth.me`, o el del sistema sin sesión (RF-51).
- **`pages/SystemPage.vue`** (`/admin/system`, con `system:manage`) tiene cuatro pestañas:
  - Acceso;
  - SSO, con la URL de retorno para copiar, el botón de login de prueba y su resultado;
  - Almacenamiento, con el bloqueo y el número de archivos;
  - Apariencia, con una vista previa en claro y en oscuro sobre una muestra de componentes, y «Restablecer».
  - Los componentes van en `features/system/`.
- **Navegación:** «Sistema» en Configuración, con `system:manage`.
- **Mi cuenta:** un selector Claro / Oscuro / Sistema.
- **Login:** el enlace «Acceso de emergencia» cuando `emergency_enabled`.
- **Textos:** `system.*` y los errores nuevos en `es.json`.
- **Vitest:**
  - el contraste de 4,5:1 de cada combinación en claro y en oscuro (RF-42), calculado de oklch a sRGB en el test;
  - el CSS generado para la apariencia inicial;
  - cada pestaña con sus rechazos;
  - el modo oscuro;
  - el enlace de emergencia.
