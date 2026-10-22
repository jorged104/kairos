# Entorno de previsualización

Cada push a `main` se despliega solo en el servidor de casa ([spec 002](../../docs/specs/002-entorno-previsualizacion/spec.md)).

```
push a main ─► GitHub Actions ─ test (GitHub) ─► deploy (runner en el servidor) ─► deploy.sh
Internet ─► Cloudflare ─► túnel ─► Traefik ─► web (nginx) ─/health─► api ─► db
```

- [`compose.yml`](compose.yml): proyecto `kairos-preview` con red interna propia. Solo `web` se une a la red de Traefik y ningún servicio publica puertos.
- [`deploy.sh`](deploy.sh): construye, migra, arranca y comprueba que responde. Si no lo hace en 60 s, vuelve a la última versión buena.
- [`../../.github/workflows/preview.yml`](../../.github/workflows/preview.yml): tests y despliegue.

> El repo es público y los logs de Actions también. El hostname y las contraseñas solo viven en
> `~/kairos/preview.env` en el servidor: nunca los pongas en el repo, en secretos de GitHub ni en
> mensajes de commit.

## Puesta en marcha (una sola vez)

Todo se hace en el servidor salvo los pasos 5 y 6.

### 1. Requisitos
- Docker con Compose v2, arrancando con el sistema: `sudo systemctl enable docker`.
- Traefik **v3.6 o superior** si el servidor usa Docker 29+. Las versiones anteriores no
  consiguen hablar con la API de Docker y dejan de ver contenedores.

### 2. Usuario dedicado para el runner
```bash
sudo useradd -m -s /bin/bash kairos-runner
sudo usermod -aG docker kairos-runner
```
El grupo `docker` equivale a root en el servidor. Por eso el workflow solo se ejecuta con push a `main` (ver paso 6).

### 3. Runner de GitHub Actions
1. En GitHub, ve a **Settings → Actions → Runners → New self-hosted runner** y elige **Linux / x64**.
2. Como `kairos-runner` (`sudo -iu kairos-runner`), ejecuta los comandos de *Download* que muestra la página.
3. En el paso *Configure*, añade nombre y etiqueta:
   ```bash
   ./config.sh --url https://github.com/jorged104/kairos --token <TOKEN_DE_LA_PÁGINA> \
     --name kairos-preview --labels kairos-preview --unattended
   ```
4. Instálalo como servicio para que sobreviva a los reinicios. Hazlo desde tu usuario con sudo, dentro de `~kairos-runner/actions-runner`:
   ```bash
   sudo ./svc.sh install kairos-runner
   sudo ./svc.sh start
   ```
5. En **Settings → Actions → Runners**, el runner debe aparecer como *Idle*.

### 4. Fichero de entorno
```bash
sudo -iu kairos-runner
mkdir -p ~/kairos && cd ~/kairos
curl -fsSLo preview.env https://raw.githubusercontent.com/jorged104/kairos/main/deploy/preview/preview.env.example
chmod 600 preview.env
nano preview.env
```
Rellena `PREVIEW_HOST`, `POSTGRES_PASSWORD` y los dos valores de Traefik. Para averiguarlos:
- **Red de Traefik:** `docker inspect <contenedor-traefik> -f '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'`
- **Entrypoint:** es el nombre que usa tu configuración de Traefik para el puerto al que llega el túnel. Por ejemplo, `--entrypoints.web.address=:80` → `web`. Usa el mismo que tienen las labels de Home Assistant.

### 5. Hostname en Cloudflare
En **Zero Trust → Networks → Tunnels → (tu túnel) → Public Hostname**, añade el hostname de
`PREVIEW_HOST` con el mismo servicio que usan tus otros hostnames (Traefik). Si ya tienes un
comodín `*.tu-dominio` hacia Traefik, no hace falta hacer nada.

### 6. Proteger el runner de forks
En **Settings → Actions → General → Fork pull request workflows from outside collaborators**,
elige **Require approval for all external contributors**.

### 7. Primer despliegue
Haz un push a `main`, o relanza el último workflow desde la pestaña **Actions**. Cuando termine con ✓:
```bash
cat ~/kairos/last-good          # commit desplegado
curl -s https://<PREVIEW_HOST>/health/ready
```

## Operación

| Qué | Cómo |
|---|---|
| Ver el resultado de un despliegue | Pestaña **Actions** en GitHub, o el ✓/✗ junto al commit |
| Relanzar un despliegue | **Actions → Previsualización → Re-run jobs** |
| Logs de los servicios | `docker compose -p kairos-preview logs -f api web` |
| Versión desplegada | `cat ~kairos-runner/kairos/last-good` |
| Parar el entorno | `docker compose -p kairos-preview stop` |
| Borrar los datos | `docker volume rm kairos-preview_pgdata`, con el entorno parado |

## Limitaciones conocidas
- **Downtime breve:** al cambiar de versión hay unos segundos sin servicio mientras se recrean los contenedores y Traefik los detecta.
- **Pushes seguidos:** GitHub mantiene en cola un solo despliegue pendiente. Con tres pushes seguidos se despliega el último y el del medio se cancela.
- **Servidor apagado:** GitHub descarta los jobs que llevan más de 24 h en cola. Si el servidor estuvo apagado más tiempo, relanza el workflow a mano.
- **Migraciones:** deben dejar funcionando la versión anterior de la API, porque la vuelta atrás no deshace migraciones (ver `AGENTS.md`).
