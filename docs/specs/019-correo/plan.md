# Plan — Spec 019: correo, invitaciones y recuperación de contraseña

## Enfoque
- **Envío SMTP con la biblioteca estándar de Go** (`net/smtp` y `crypto/tls`), sin dependencias nuevas.
  - Tres modos de seguridad: ninguna, STARTTLS y TLS directo.
  - Mensaje `multipart/alternative` con una parte de texto y otra HTML.
  - Cada fallo indica su paso: `connect`, `tls`, `auth` o `send` (RF-7).
- **Configuración en `system_settings`**, en la misma fila y con el mismo `Holder` de la spec 015.
  - La contraseña del servidor va cifrada con la misma caja (RF-4).
  - Un cambio sube la versión, y las demás instancias lo recogen en menos de 5 s (RF-9).
- **Enlaces en una tabla nueva `user_tokens`:**
  - propósito `invite` o `reset`;
  - se guarda solo la huella `sha256` (RF-38);
  - caducidad, uso y anulación.
  - El secreto son 32 bytes aleatorios en base64url (RF-37).
- **El enlace va en el fragmento de la URL:** `PUBLIC_URL/password#t=<secreto>`. El navegador nunca envía el fragmento, así que no llega a los logs de nginx ni de la API (RF-39). La página lo envía a la API en el cuerpo de un POST.
- **Paquete nuevo `accounts`** para los flujos de invitación y recuperación. Usa `auth` (hasher y reglas de contraseña), el correo y la configuración viva. `users` lo llama en el alta.

## Envío sin esperas
- **Invitación (alta y reenvío):**
  - el correo se envía en segundo plano y la respuesta espera como mucho 1,5 s;
  - si el correo se envió, se responde `sent: true`;
  - si falló, no está configurado o no terminó a tiempo, se responde `sent: false` con el `url` para copiar (RF-13, RF-21).
- **Recuperación pedida desde el login:** siempre en segundo plano, con la misma respuesta haya o no correo (RF-25, RF-26).
- **Aviso de contraseña cambiada:** en segundo plano (RF-31).

## Datos (migración `00017_correo.sql`, solo aditiva)
- **`system_settings`:**
  - `mail_enabled`;
  - `smtp_host`, `smtp_port`, `smtp_security`, `smtp_username`;
  - `smtp_password` (cifrada);
  - `mail_from_email`, `mail_from_name`.
- **`user_tokens`:**
  - `id`, `user_id`, `purpose`, `token_hash` (único);
  - `created_by`, `created_at`, `expires_at`, `used_at`, `revoked_at`.

## API
- **Sistema:**
  - `PUT /api/system/settings/mail`, con `system:manage`;
  - `POST /api/system/settings/mail/test`, que envía a la dirección del administrador (RF-6).
- **Usuarios:**
  - `POST /api/users` ya no acepta contraseña, y su respuesta añade `invitation { sent, url?, expires_at? }`;
  - `POST /api/users/{id}/invitation`: reenviar, con el permiso de dar de alta a esa persona (RF-20);
  - `POST /api/users/{id}/password-link`: enlace de recuperación, con `passwords:set` (RF-32);
  - `PUT /api/users/{id}/password` desaparece (RF-34).
- **Públicas:**
  - `POST /api/auth/password/forgot { email }`: responde siempre `202` (RF-25, RF-27);
  - `POST /api/auth/password/check { token }`: devuelve `{ purpose, email }` o `invalid_link`;
  - `POST /api/auth/password/reset { token, password }` (RF-16, RF-29).
- **Usuario:** el `User` gana `invitation { expires_at, expired }` mientras tenga una invitación sin usar y no tenga contraseña (RF-19).
- **Modo solo SSO:**
  - la invitación es un aviso con el enlace al login y no crea enlace (RF-15);
  - la recuperación solo atiende al administrador inicial (RF-24).
- **Al desactivar a una persona** o darla de baja se anulan sus enlaces (RF-42).

## Desarrollo
Nuevo profile `mail` en `docker-compose`, con Mailpit (SMTP en el puerto 1025 y bandeja web en `localhost:8025`) y `make up-mail`. Solo para el entorno local; nunca se despliega.

## Frontend
- **Sistema:** pestaña «Correo» (`MailSettings`) con el formulario, el interruptor y «Enviar correo de prueba».
- **Usuarios:**
  - el alta va sin contraseña y, si el correo no salió, muestra el enlace para copiar;
  - la ficha muestra la invitación pendiente, con «Reenviar invitación» y «Enviar enlace de nueva contraseña»;
  - el directorio marca a quien tiene una invitación pendiente.
- **Público:**
  - en el login, «¿Olvidaste tu contraseña?», que en modo solo SSO va dentro del acceso de emergencia;
  - páginas `/forgot-password` y `/password`, que lee el enlace del fragmento.

## Tests
- **apitest:**
  - un servidor SMTP falso dentro del test (`net.Listen`), que guarda los mensajes y puede rechazar la autenticación;
  - configuración y prueba de envío, con el paso del fallo;
  - invitación con y sin correo, en cada modo de acceso;
  - recuperación: misma respuesta, límite de 3 por hora, solo vale el último enlace, caducidad, uso único y personas desactivadas;
  - permisos y auditoría;
  - filas de la matriz.
- **Vitest:** las pantallas citadas.
