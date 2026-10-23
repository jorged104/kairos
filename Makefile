-include .env
export

# Evita que Git Bash en Windows reescriba rutas como /src al llamar a docker.
export MSYS_NO_PATHCONV := 1

DB_WAIT_SECONDS := 60
SQLC_IMAGE := sqlc/sqlc:1.30.0

.PHONY: up up-sso db api web down migrate sqlc test test-api test-web dev-api dev-web

up: db ## Arranca BD, API y frontend; la API solo arranca si la BD está sana
	docker compose up -d --build --no-deps api web

up-sso: db ## Como up, más Keycloak de prueba (profile sso) en localhost:8180
	docker compose --profile sso up -d --build --no-deps keycloak api web

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
