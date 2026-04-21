ENV_FILE ?= /home/runner/work/scrum/scrum/scrumX/infra/.env
COMPOSE_FILE ?= /home/runner/work/scrum/scrum/scrumX/infra/docker-compose.yml

.PHONY: up down logs

$(ENV_FILE):
	cp /home/runner/work/scrum/scrum/scrumX/infra/.env.example $(ENV_FILE)

up: $(ENV_FILE)
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up --build

down:
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) down

logs:
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) logs -f
