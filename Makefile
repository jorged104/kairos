-include .env
export

DB_WAIT_SECONDS := 60

.PHONY: db down

db: ## Arranca PostgreSQL y espera a que esté sano (máx. 60 s)
	@docker compose up -d --wait --wait-timeout $(DB_WAIT_SECONDS) db || \
		{ echo "La base de datos no respondió en $(DB_WAIT_SECONDS) s" >&2; exit 1; }

down: ## Para y elimina los contenedores
	docker compose down
