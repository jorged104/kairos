#!/usr/bin/env bash
# Despliega el commit actual en el entorno de previsualización (spec 002).
# Lo ejecuta el job "deploy" del workflow en el runner del servidor.
#
# Los logs de GitHub Actions son públicos: este script no imprime nunca el
# hostname ni credenciales, ni ejecuta "compose config" (RF-17).
set -euo pipefail

READY_TIMEOUT_SECONDS=${READY_TIMEOUT_SECONDS:-60}
STATE_DIR=${KAIROS_STATE_DIR:-$HOME/kairos}
ENV_FILE=${KAIROS_PREVIEW_ENV:-$STATE_DIR/preview.env}
LAST_GOOD_FILE=$STATE_DIR/last-good
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
COMPOSE_FILE=$ROOT/deploy/preview/compose.yml

log() { printf '[deploy %(%T)T] %s\n' -1 "$*"; }
fail() {
	printf '[deploy %(%T)T] ERROR: %s\n' -1 "$*" >&2
	exit 1
}

compose() {
	docker compose -p kairos-preview -f "$COMPOSE_FILE" --env-file "$ENV_FILE" "$@"
}

# Waits until the whole path web -> api -> db answers, or the timeout expires.
wait_ready() {
	local deadline=$((SECONDS + READY_TIMEOUT_SECONDS))
	while :; do
		if compose exec -T web wget -q -T 3 -O /dev/null http://127.0.0.1/health/ready 2>/dev/null; then
			return 0
		fi
		# Each attempt takes a couple of seconds; stop before overshooting the limit.
		((SECONDS + 3 < deadline)) || return 1
		sleep 1
	done
}

# Keeps only the images of the current and previous good versions.
prune_images() {
	local repo tag
	for repo in kairos-api kairos-web kairos-migrate; do
		docker image ls "$repo" --format '{{.Tag}}' | while read -r tag; do
			if [[ $tag != "$TAG" && $tag != "$PREV" ]]; then
				docker image rm "$repo:$tag" >/dev/null 2>&1 || true
			fi
		done
	done
}

[[ -f $ENV_FILE ]] || fail "no existe el fichero de entorno del servidor (ver deploy/preview/README.md)"
mkdir -p "$STATE_DIR"

TAG=${TAG:-$(git -C "$ROOT" rev-parse --short=12 HEAD)}
PREV=$(cat "$LAST_GOOD_FILE" 2>/dev/null || true)
export TAG

log "versión nueva: $TAG · última buena: ${PREV:-ninguna}"

log "1/5 construyendo imágenes"
compose build api web migrate || fail "la construcción falló; no se despliega"

log "2/5 base de datos"
compose up -d --wait db || fail "la base de datos no arrancó; no se despliega"

log "3/5 migraciones"
compose run --rm migrate || fail "las migraciones fallaron; sigue en marcha ${PREV:-la versión anterior}"

log "4/5 arrancando $TAG"
compose up -d --no-deps api web

log "5/5 comprobando disponibilidad (máx. ${READY_TIMEOUT_SECONDS} s)"
if wait_ready; then
	echo "$TAG" >"$LAST_GOOD_FILE"
	prune_images
	log "desplegado $TAG"
	exit 0
fi

log "la versión $TAG no está disponible"
if [[ -n $PREV ]]; then
	log "volviendo a $PREV"
	TAG=$PREV compose up -d --no-deps api web
	if wait_ready; then
		log "restaurada $PREV"
	else
		log "AVISO: $PREV tampoco responde"
	fi
fi
fail "despliegue de $TAG fallido"
