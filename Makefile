-include .env
export

# Evita que Git Bash en Windows reescriba rutas como /src al llamar a docker.
export MSYS_NO_PATHCONV := 1

DB_WAIT_SECONDS := 60
SQLC_IMAGE := sqlc/sqlc:1.30.0

.PHONY: db down sqlc test-api

db: ## Arranca PostgreSQL y espera a que esté sano (máx. 60 s)
	@docker compose up -d --wait --wait-timeout $(DB_WAIT_SECONDS) db || \
		{ echo "La base de datos no respondió en $(DB_WAIT_SECONDS) s" >&2; exit 1; }

down: ## Para y elimina los contenedores
	docker compose down

sqlc: ## Regenera el código de sqlc (en contenedor: sqlc necesita cgo)
	docker run --rm -v "$(CURDIR)/backend:/src" -w /src $(SQLC_IMAGE) generate

test-api: ## Tests del backend
	cd backend && go test ./...
