.PHONY: run stop logs restart autopay autopay-bind dev server

MONTHS ?= 1

run:
	docker compose up -d

stop:
	docker compose down

logs:
	docker compose logs -f

restart:
	docker compose down && docker compose up -d --build --force-recreate --remove-orphans

autopay:
	go run ./other/microservice/autopay

autopay-bind:
	go run ./other/microservice/autopay -user $(USER) -months $(MONTHS)

dev:
	docker compose -f docker-compose-min.yml up -d

server:
	git pull && docker compose pull && make restart
