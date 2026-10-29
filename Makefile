-include .env
export

# Evita que Git Bash en Windows reescriba rutas como /src al llamar a docker.
export MSYS_NO_PATHCONV := 1

DB_WAIT_SECONDS := 60
SQLC_IMAGE := sqlc/sqlc:1.30.0
S3_BUCKET := kairos

.PHONY: up up-sso up-s3 db api web down migrate sqlc test test-api test-web dev-api dev-web

up: db ## Arranca BD, API y frontend; la API solo arranca si la BD está sana
	docker compose up -d --build --no-deps api web

up-sso: db ## Como up, más Keycloak de prueba (profile sso) en localhost:8180
	docker compose --profile sso up -d --build --no-deps keycloak api web

up-s3: db ## Como up, con los archivos de las páginas en S3 (SeaweedFS, profile s3) en localhost:8333
	docker compose --profile s3 up -d s3
	@for i in $$(seq 1 30); do 		docker compose exec -T s3 sh -c 'echo s3.bucket.list | weed shell | grep -qw $(S3_BUCKET) || echo "s3.bucket.create -name $(S3_BUCKET)" | weed shell' >/dev/null 2>&1 && exit 0; 		sleep 2; 	done; echo 'SeaweedFS no respondió en 60 s' >&2; exit 1
	FILES_S3_ENDPOINT_CONTAINER=s3:8333 FILES_S3_BUCKET=$(S3_BUCKET) FILES_S3_ACCESS_KEY=kairos-dev 		FILES_S3_SECRET_KEY=dev-only-kairos-s3-secret docker compose --profile s3 up -d --build --no-deps api web

db: ## Arranca PostgreSQL y espera a que esté sano (máx. 60 s)
	@docker compose up -d --wait --wait-timeout $(DB_WAIT_SECONDS) db || \
		{ printf 'La base de datos no respondi\303\263 en %s s\n' "$(DB_WAIT_SECONDS)" >&2; exit 1; }

api: ## Arranca la API en contenedor sin arrastrar la BD (falla si la BD no está)
	docker compose up -d --build --no-deps api

web: ## Arranca el frontend (Vite) en contenedor
	docker compose up -d --build --no-deps web

down: ## Para y elimina los contenedores (también Keycloak)
	docker compose --profile sso down

migrate: ## Aplica las migraciones pendientes (nunca se ejecuta al arrancar)
	@cd backend && go tool goose -dir internal/db/migrations postgres "$$DATABASE_URL" up || \
		{ echo "No se pudieron aplicar las migraciones" >&2; exit 1; }

sqlc: ## Regenera el código de sqlc (en contenedor: sqlc necesita cgo)
	docker run --rm -v "$(CURDIR)/backend:/src" -w /src $(SQLC_IMAGE) generate

test: test-api test-web ## Tests de backend y frontend

test-api: ## Tests del backend
	cd backend && go test ./...

test-web: ## Tests del frontend
	cd frontend && npm test -- --run

dev-api: ## API en el host contra la BD en contenedor
	cd backend && go run ./cmd/api

dev-web: ## Vite en el host, con proxy a la API en API_PROXY_TARGET
	cd frontend && npm run dev
