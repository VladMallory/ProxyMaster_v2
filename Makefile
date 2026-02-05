.PHONY: run windows run2 backup-db backup-db-gz backup-list backup-info backup-restore backup-cleanup db-only db-stop docker-build docker dc dcd dev dev-stop docker-build-linux docker-linux gosec list
binary=ProxyMaster_v2
cmdMacosAndLinux=./cmd/myapp/main.go
cmdWindows=.\cmd\myapp\main.go

run:
	@clear
	@go run $(cmdMacosAndLinux)

windows:
	go run $(cmdWindows)

run2:
	go run .\cmd\botTest\main.go

# ==================
# БЕКАПЫ БАЗЫ ДАННЫХ
# ==================

# Создать бекап базы данных
backup-db:
	@mkdir -p other/backups
	@docker exec users_postgres pg_dump -U user -d usersdb > other/backups/usersdb_backup_$$(date +%Y%m%d_%H%M%S).sql
	@echo "✓ Бекап успешно создан в other/backups/"

# Создать сжатый бекап
backup-db-gz:
	@mkdir -p other/backups
	@docker exec users_postgres pg_dump -U user -d usersdb | gzip > other/backups/usersdb_backup_$$(date +%Y%m%d_%H%M%S).sql.gz
	@echo "✓ Сжатый бекап создан в other/backups/"

# Показать список всех бекапов
backup-list:
	@echo "📁 Список бекапов в other/backups/:"
	@ls -lh other/backups/ 2>/dev/null || echo "  Бекапов нет"

# Показать информацию о последнем бекапе
backup-info:
	@echo "📊 Последний бекап:"
	@ls -lh other/backups/ | tail -2 || echo "  Бекапов нет"

# Восстановить базу из бекапа (указать имя файла)
# Использование: make backup-restore FILE=usersdb_backup_20260115_193758.sql
backup-restore:
	@if [ -z "$(FILE)" ]; then \
		echo "❌ Нужно указать имя файла бекапа"; \
		echo "Использование: make backup-restore FILE=usersdb_backup_YYYYMMDD_HHMMSS.sql"; \
		echo ""; \
		make backup-list; \
	else \
		if [ ! -f "other/backups/$(FILE)" ]; then \
			echo "❌ Файл other/backups/$(FILE) не найден"; \
		else \
			echo "⚠️  ВНИМАНИЕ: Это перезапишет текущую базу данных!"; \
			echo "Восстанавливаю из: $(FILE)"; \
			docker exec -i users_postgres psql -U user -d usersdb < other/backups/$(FILE) && \
			echo "✓ База успешно восстановлена из $(FILE)"; \
		fi; \
	fi

# Запустить ТОЛЬКО PostgreSQL (без приложения)
db-only: db-stop
	@echo "🔄 Запуск только PostgreSQL..."
	@docker compose -f docker-compose.dev.yml up -d postgres
	@echo "⏳ Ожидание инициализации БД..."
	@sleep 3
	@docker exec users_postgres pg_isready -U user -d usersdb > /dev/null 2>&1 && \
	echo "✓ База данных готова к работе!" || echo "⚠️  Проверьте статус: docker ps"

# Остановить только PostgreSQL
db-stop:
	@docker compose -f docker-compose.dev.yml down postgres 2>/dev/null || true
	@echo "✓ PostgreSQL остановлен"

# docker
# натив
docker-build:
	docker build -t proxymaster_v2 .

docker: docker-build
	docker run --env-file .env proxymaster_v2

# Запустить докер без отображения логов
dc: dcd
	docker compose up -d --build

# Запустить с показом логгов
dcl: dc
	docker compose logs -f

# Остановить докер
dcd:
	docker compose down

server: 
	git pull
	make dcl

update:
	docker compose pull
	docker compose down --remove-orphans
	docker compose build --no-cache
	docker compose up -d --force-recreate
	docker compose logs -f

# Запуск dev окружения (без Go приложения)
dev: dev-stop
	docker compose -f docker-compose.dev.yml up -d --build

# Остановить dev окружение
dev-stop:
	docker compose -f docker-compose.dev.yml down

# эмуляция под linux
docker-build-linux:
	docker build --platform linux/amd64 -t proxymaster_v2 .

docker-linux: docker-build-linux
	docker run --platform linux/amd64 --env-file .env proxymaster_v2

# Проверки и прочее
gosec:
	@clear
	gosec ./...

list:
	@clear
	golangci-lint run
