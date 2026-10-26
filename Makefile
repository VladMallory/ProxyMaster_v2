run:
	docker compose up -d

stop:
	docker compose down

logs: 
	docker compose logs -f

restart:
	docker compose down && docker compose up -d --build --force-recreate --remove-orphans
