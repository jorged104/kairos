# Instalación local

Kairos completo en tu máquina, para probarlo o desarrollar. Todo corre en contenedores; no hace falta instalar PostgreSQL.

## Requisitos

- Docker con Compose v2.
- `make`. En Windows: `winget install ezwinports.make`, o trabaja desde WSL.
- Solo para desarrollar fuera de contenedores, o para `make migrate` y `make test`:
  - Go ≥ 1.27 (con Go ≥ 1.21 la versión correcta se descarga sola);
  - Node ≥ 22;
  - [uv](https://docs.astral.sh/uv/), para los tests del agente de informes (`make test-agent`).

## Primera vez

```bash
git clone https://github.com/jorged104/kairos.git
cd kairos
cp .env.example .env
make up        # BD + API + agente de informes + frontend; espera hasta 60 s a que la BD esté sana
make migrate   # crea las tablas
```

Abre http://localhost:5173 y **regístrate con el correo `admin@kairos.local`**. Es el de `BOOTSTRAP_ADMIN_EMAIL` en `.env`, así que esa cuenta se convierte en administradora. Cualquier otra cuenta que registres después entra con el rol `member`.

| Servicio | Dirección |
|---|---|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| PostgreSQL | `localhost:5432` (usuario, contraseña y BD `openscum`) |
| Agente de informes con IA | `localhost:8000` (solo lo usa la API) |

## Servicios opcionales

Cada uno arranca lo mismo que `make up` y además un servicio de prueba:

| Comando | Qué añade | Cómo usarlo |
|---|---|---|
| `make up-sso` | Keycloak en http://localhost:8180 | Pon `AUTH_MODE=both` en `.env`. Usuarios `ana`, `bruno`, `carla` y `admin`, contraseña `kairos-dev-123` |
| `make up-s3` | SeaweedFS en `localhost:8333` | Los archivos de las páginas se guardan en S3 en vez de en la BD |
| `make up-mail` | Mailpit: SMTP en `localhost:1025`, bandeja en http://localhost:8025 | En Sistema → Correo: servidor `mail`, puerto `1025`, seguridad «Ninguna» |

Todas las credenciales de `.env.example` y de `scripts/` son solo de desarrollo.

## Informes con IA

El agente que escribe los informes (spec 020) arranca con `make up`. Para usarlo, en **Sistema → Informes con IA** indica un servicio compatible con la API de OpenAI (dirección base, modelo y, si la pide, clave) y pulsa «Probar conexión». El modelo tiene que admitir llamadas a herramientas. Sirve también un modelo local con una API compatible, por ejemplo Ollama (`http://host.docker.internal:11434/v1`).

## Desarrollar con la API o el frontend en el host

Útil para recargar al instante sin reconstruir contenedores:

```bash
make db        # solo PostgreSQL
make dev-api   # API con go run, contra la BD en contenedor
make dev-web   # Vite en http://localhost:5173, con proxy a la API
```

Con `make dev-api`, el servidor de correo de Mailpit es `localhost` en lugar de `mail`.

Para generar informes con IA con `make dev-api`, arranca el agente con `make agent` y descomenta `AGENT_URL` y `AGENT_CALLBACK_URL` en `.env`: la API en el host lo encuentra en `localhost:8000` y él vuelve a la API por `host.docker.internal`.

## Comandos

```bash
make up          # BD + API + agente de informes + frontend en contenedores
make db          # solo PostgreSQL
make api         # solo la API (sin BD disponible, sale con error)
make web         # solo el frontend
make down        # para y elimina los contenedores (los datos se conservan)
make migrate     # aplica las migraciones pendientes
make sqlc        # regenera el código de sqlc (en contenedor)
make agent       # solo el agente de informes con IA
make test        # tests de backend, frontend y agente (test-api / test-web / test-agent por separado)
```

Los tests de integración del backend necesitan `TEST_DATABASE_URL` (ya está en `.env.example`) y la BD en marcha; sin ella se omiten.

## Actualizar tu copia local

```bash
git pull
make up        # reconstruye las imágenes con el código nuevo
make migrate
```

## Empezar de cero

```bash
make down
docker volume ls | grep pgdata     # el prefijo es el nombre de la carpeta, p. ej. kairos_pgdata
docker volume rm kairos_pgdata     # borra todos los datos
make up && make migrate
```

## Problemas frecuentes

| Síntoma | Causa y solución |
|---|---|
| `La base de datos no respondió en 60 s` | Docker no está arrancado, o el puerto 5432 lo usa otro PostgreSQL. Cambia `POSTGRES_PORT` en `.env` |
| La API sale nada más arrancar | Falta una variable obligatoria; el log lo dice: `docker compose logs api` |
| Pantalla en blanco o errores 500 tras un `git pull` | Faltan migraciones: `make migrate` |
| `make` no existe en Windows | `winget install ezwinports.make`, o usa WSL |
