# Instalación en un servidor

Kairos en marcha para un equipo, en un servidor Linux con Docker. Usa [`deploy/servidor/compose.yml`](../../deploy/servidor/compose.yml): las imágenes se construyen en el propio servidor a partir del código.

## Requisitos

- Linux con Docker y Compose v2, arrancando con el sistema (`sudo systemctl enable docker`).
- Al menos 2 GB de RAM y 10 GB de disco.
- Un nombre de dominio que apunte al servidor, por ejemplo `kairos.example.com`.
- Un **proxy inverso con HTTPS** delante. Kairos exige HTTPS fuera de desarrollo, porque la cookie de sesión solo viaja por conexiones seguras. Aquí se usa Caddy, que obtiene el certificado solo; vale cualquier otro (nginx, Traefik).
- Opcional: un servidor SMTP para las invitaciones y la recuperación de contraseña, un proveedor de identidad OIDC para el SSO, un almacenamiento compatible con S3 para los archivos y un servicio de IA compatible con la API de OpenAI para los informes.

## 1. Descargar el código

```bash
sudo mkdir -p /opt/kairos && sudo chown "$USER" /opt/kairos
git clone https://github.com/jorged104/kairos.git /opt/kairos
cd /opt/kairos
```

## 2. Fichero de entorno

```bash
sudo mkdir -p /etc/kairos
sudo cp deploy/servidor/kairos.env.example /etc/kairos/kairos.env
sudo chown "$USER" /etc/kairos/kairos.env && chmod 600 /etc/kairos/kairos.env
openssl rand -base64 48     # cópialo en SESSION_SECRET
nano /etc/kairos/kairos.env
```

Como mínimo rellena:

| Variable | Valor |
|---|---|
| `PUBLIC_URL` | `https://kairos.example.com`, tal como lo escribirá la gente en el navegador |
| `POSTGRES_PASSWORD` | Una contraseña larga sin `@ : / ? # %` |
| `SESSION_SECRET` | El resultado de `openssl rand -base64 48` |
| `BOOTSTRAP_ADMIN_EMAIL` | Tu correo: con él serás el primer administrador |
| `AUTH_MODE` | `local` (usuario y contraseña), `oidc` (solo SSO) o `both` |

> **Guarda una copia de `SESSION_SECRET` en un lugar seguro.** Además de firmar las sesiones, cifra las contraseñas de SMTP, SSO y S3 que se guardan desde la pantalla Sistema. Si la pierdes o la cambias, habrá que volver a escribirlas todas.

El resto de variables está en [Configuración](configuracion.md).

## 3. Construir, migrar y arrancar

Para no repetir la línea larga, define un alias (añádelo a tu `~/.bashrc`):

```bash
alias kairos='docker compose -p kairos -f /opt/kairos/deploy/servidor/compose.yml --env-file /etc/kairos/kairos.env'
```

```bash
kairos build api web migrate agent   # unos minutos la primera vez
kairos up -d --wait db
kairos run --rm migrate           # crea las tablas
kairos up -d agent api web
curl -s http://127.0.0.1:8081/health/ready    # {"status":"ok",...}
```

## 4. HTTPS con Caddy

Instala Caddy ([instrucciones oficiales](https://caddyserver.com/docs/install)) y deja `/etc/caddy/Caddyfile` así:

```
kairos.example.com {
    reverse_proxy 127.0.0.1:8081
}
```

```bash
sudo systemctl reload caddy
```

Caddy pide el certificado a Let's Encrypt la primera vez; los puertos 80 y 443 deben estar abiertos hacia el servidor. Las conexiones en tiempo real (`/ws`) pasan sin configuración extra.

Si usas otro proxy, comprueba que:
- reenvía todo el tráfico de tu dominio a `127.0.0.1:8081`;
- admite WebSocket en `/ws`;
- envía `X-Forwarded-For` y `X-Forwarded-Proto`;
- **no guarda en sus logs las rutas `/api/mcp/`**, porque contienen el secreto de cada conexión de IA (con Caddy no hay log de accesos salvo que lo actives).

## 5. Primer acceso

1. Abre `https://kairos.example.com`.
2. Según el modo:
   - **`local` o `both`**: regístrate con el correo de `BOOTSTRAP_ADMIN_EMAIL`;
   - **`oidc`**: entra con el SSO usando ese mismo correo.
   Esa cuenta queda como administradora.
3. En **Sistema → Acceso**, desactiva el registro libre si no lo quieres abierto. Desde entonces, las cuentas nuevas las crea un administrador en **Usuarios**.
4. En **Sistema → Correo**, configura el SMTP y pulsa «Enviar correo de prueba». Sin correo, las invitaciones funcionan igual: Kairos te muestra el enlace para que lo copies.
5. Si quieres informes con IA, en **Sistema → Informes con IA** indica el servicio (dirección base, modelo y clave) y pulsa «Probar conexión». El servidor tiene que poder salir hacia ese servicio. Los datos que la IA consulta se envían a él: elige uno en el que tu organización confíe.
6. Revisa el resto de pestañas de **Sistema** (SSO, almacenamiento de archivos, apariencia). Lo que se guarda ahí tiene prioridad sobre las variables de entorno.

> **Elige el almacenamiento de archivos antes de empezar a usar Kairos.** En cuanto hay un archivo guardado, el almacenamiento queda bloqueado y no se puede cambiar.

## 6. Copias de seguridad

Todo lo importante está en PostgreSQL (y en el bucket de S3, si lo usas). Una copia diaria con cron:

```bash
sudo mkdir -p /var/backups/kairos && sudo chown "$USER" /var/backups/kairos
crontab -e
```
```
30 3 * * * docker compose -p kairos -f /opt/kairos/deploy/servidor/compose.yml --env-file /etc/kairos/kairos.env exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB"' > /var/backups/kairos/kairos-$(date +\%F).dump && find /var/backups/kairos -name '*.dump' -mtime +14 -delete
```

Para restaurar una copia sobre una instalación parada:

```bash
kairos stop api web
kairos exec -T db sh -c 'dropdb -U "$POSTGRES_USER" --if-exists "$POSTGRES_DB" && createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'
kairos exec -T db sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < /var/backups/kairos/kairos-AAAA-MM-DD.dump
kairos up -d agent api web
```

Guarda las copias también fuera del servidor, junto con `/etc/kairos/kairos.env`.

## Operación

| Qué | Cómo |
|---|---|
| Estado de los servicios | `kairos ps` |
| Logs | `kairos logs -f api web agent` |
| ¿Responde? | `curl -s http://127.0.0.1:8081/health/ready` |
| Reiniciar | `kairos restart agent api web` |
| Parar | `kairos stop` |
| Actualizar | [Actualización](actualizacion.md) |

## Problemas frecuentes

| Síntoma | Causa y solución |
|---|---|
| La API se reinicia en bucle | Falta o es inválida una variable; `kairos logs api` dice cuál |
| Entras y vuelves a la pantalla de login | El sitio no va por HTTPS, o `PUBLIC_URL` no coincide exactamente con la dirección del navegador |
| Errores 500 tras actualizar | No se aplicaron las migraciones: `kairos run --rm migrate` |
| No llega el tiempo real (notificaciones, cambios de otros) | El proxy no deja pasar WebSocket en `/ws` |
| «Probar conexión» del servicio de IA falla | El motivo exacto que dio el servicio (cuota agotada, dirección mal escrita, parámetro no admitido…) queda en `kairos logs api`, en la línea «prueba del servicio de IA fallida». La dirección base termina normalmente en `/v1` |
| «El agente de informes no está disponible» | `kairos ps` muestra si `agent` está en marcha; `kairos logs agent` dice por qué falla |
| El SSO falla al volver del proveedor | La URL de retorno registrada en el IdP debe ser `<PUBLIC_URL>/api/auth/oidc/callback` |
