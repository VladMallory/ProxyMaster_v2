run:
	docker compose up -d

stop:
	docker compose down

logs: 
	docker compose logs -f

restart:
	docker compose down && docker compose up -d --build --force-recreate --remove-orphans

dev:
	docker compose -f docker-compose-min.yml up -d 

server: 
	git pull && docker compose pull && make restart
