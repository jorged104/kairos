# Actualización

Cómo pasar una instalación en servidor a una versión nueva. Para tu copia local basta con `git pull`, `make up` y `make migrate`.

Los pasos usan el alias `kairos` de la [instalación en un servidor](instalacion-servidor.md#3-construir-migrar-y-arrancar).

## Antes de actualizar

1. **Lee qué cambia.** Revisa los commits o las PR fusionadas desde tu versión:
   ```bash
   cd /opt/kairos
   git fetch
   git log --oneline HEAD..origin/main
   ```
2. **Busca variables nuevas.** Si la versión añade variables obligatorias, añádelas a `/etc/kairos/kairos.env` antes de arrancarla:
   ```bash
   git diff HEAD origin/main -- deploy/servidor/kairos.env.example
   ```
3. **Haz una copia de seguridad** de la base de datos (ver [copias de seguridad](instalacion-servidor.md#6-copias-de-seguridad)):
   ```bash
   kairos exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB"' > /var/backups/kairos/antes-de-actualizar.dump
   ```
4. **Apunta la versión actual**, por si hay que volver atrás:
   ```bash
   git rev-parse --short HEAD > /var/backups/kairos/version-anterior
   ```

## Actualizar

```bash
cd /opt/kairos
git pull --ff-only
kairos build api web migrate agent   # la versión actual sigue funcionando mientras tanto
kairos run --rm migrate           # aplica las migraciones nuevas
kairos up -d agent api web           # unos segundos sin servicio mientras se recrean
curl -s http://127.0.0.1:8081/health/ready
```

Después abre Kairos en el navegador y recarga la página (el frontend nuevo se carga al recargar).

Si `migrate` falla, **no arranques la versión nueva**: la anterior sigue en marcha. Revisa el error y, si hace falta, restaura la copia.

## Volver a la versión anterior

Las migraciones de Kairos son solo aditivas: la versión anterior de la API sigue funcionando con la base de datos ya migrada. Por eso, volver atrás es solo volver al código anterior:

```bash
cd /opt/kairos
git checkout "$(cat /var/backups/kairos/version-anterior)"
kairos build api web agent
kairos up -d agent api web
```

No deshagas migraciones con `goose down`: puede borrar datos. Si la base de datos quedó mal, restaura la copia de antes de actualizar.

Para volver a seguir `main` después: `git checkout main`.

## Fijar una versión concreta

Si prefieres no seguir `main` sino un commit concreto ya probado:

```bash
git fetch
git checkout <commit>
```

Y sigue los pasos de [Actualizar](#actualizar) desde `kairos build`.

## Mantenimiento ocasional

- **Imágenes antiguas:** cada actualización deja las imágenes viejas de Docker; libera espacio con `docker image prune`.
- **Docker y el sistema:** actualízalos con el gestor de paquetes. Después, `kairos up -d` vuelve a levantar todo; los datos están en el volumen `kairos_pgdata`.
- **PostgreSQL:** la imagen es `postgres:17-alpine`. Pasar a otra versión mayor de PostgreSQL requiere volcar y restaurar los datos; no cambies la versión en `compose.yml` sin hacerlo.
