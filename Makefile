up:
	docker compose -f orion-infra/docker-compose.yml --env-file .env up -d --build
down:
	docker compose -f orion-infra/docker-compose.yml --env-file .env down