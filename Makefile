.PHONY: run windows run2 backup-db backup-db-gz backup-list backup-info backup-restore backup-restore-clean backup-cleanup db-only db-stop docker-build docker dc dcd dev dev-stop docker-build-linux docker-linux gosec list vault-status vault-init vault-unseal vault-put-secrets vault-approle-setup site-pay
binary=ProxyMaster_v2
cmdMacosAndLinux=./cmd/myapp/main.go
cmdWindows=.\cmd\myapp\main.go

run:
	@clear
	@go run $(cmdMacosAndLinux)

windows:
	go run $(cmdWindows)

site-pay:
	caddy validate --config cmd/restAPITest/Caddyfile
	caddy reload --config cmd/restAPITest/Caddyfile --adapter caddyfile
	docker compose -f docker-compose.site.yml up -d --build
	docker logs -f site_pay

# ==================
# БЕКАПЫ БАЗЫ ДАННЫХ
# ==================

# Создать бекап базы данных
backup-db:
	@mkdir -p other/backups
	@docker exec users_postgres pg_dump -U user -d usersdb > other/backups/usersdb_backup_$$(date +%Y%m%d_%H%M%S).sql
	@echo "✓ Бекап успешно создан в other/backups/"

# Показать список всех бекапов
backup-list:
	@echo "📁 Список бекапов в other/backups/:"
	@ls -lh other/backups/ 2>/dev/null || echo "  Бекапов нет"

# Показать информацию о последнем бекапе
backup-info:
	@echo "📊 Последний бекап:"
	@ls -lh other/backups/ | tail -2 || echo "  Бекапов нет"

# Восстановить базу из бекапа (полная перезапись)
# Использование: make backup-restore FILE=backup_2026-02-05_02-00-01.sql
backup-restore:
	@if [ -z "$(FILE)" ]; then \
		echo "❌ Нужно указать имя файла бекапа"; \
		echo "Использование: make backup-restore FILE=backup_YYYY-MM-DD_HH-MM-SS.sql"; \
		echo "Или: make backup-restore FILE=usersdb_backup_YYYYMMDD_HHMMSS.sql"; \
		echo ""; \
		make backup-list; \
	else \
		if [ ! -f "other/backups/$(FILE)" ]; then \
			echo "❌ Файл other/backups/$(FILE) не найден"; \
		else \
			echo "⚠️  ВНИМАНИЕ: Это УДАЛИТ текущую базу данных и создаст новую из бекапа!"; \
			echo "Восстанавливаю из: $(FILE)"; \
			\
			echo "🗑️  Очищаю базу данных..."; \
			docker exec users_postgres psql -U user -d usersdb -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" && \
			echo "✅ База очищена"; \
			\
			echo "📥 Восстанавливаю данные из бекапа..."; \
			docker exec -i users_postgres psql -U user -d usersdb < other/backups/$(FILE) && \
			echo "✓ База успешно восстановлена из $(FILE)"; \
			\
			echo "👥 Проверьте количество пользователей: go run ./other/tools/database/editClient.go"; \
		fi; \
	fi

# Полностью заменить базу данных из бекапа (удалить старую и восстановить)
# Использование: make backup-restore-clean FILE=backup_2026-02-05_02-00-01.sql
backup-restore-clean:
	@if [ -z "$(FILE)" ]; then \
		echo "❌ Нужно указать имя файла бекапа"; \
		echo "Использование: make backup-restore-clean FILE=backup_YYYY-MM-DD_HH-MM-SS.sql"; \
		echo ""; \
		make backup-list; \
	else \
		if [ ! -f "other/backups/$(FILE)" ]; then \
			echo "❌ Файл other/backups/$(FILE) не найден"; \
		else \
			echo "⚠️  ВНИМАНИЕ: Это УДАЛИТ текущую базу данных и создаст новую из бекапа!"; \
			echo "Восстанавливаю из: $(FILE)"; \
			\
			echo "🗑️  Очищаю базу данных..."; \
			docker exec users_postgres psql -U user -d usersdb -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" && \
			echo "✅ База очищена"; \
			\
			echo "📥 Восстанавливаю данные из бекапа..."; \
			docker exec -i users_postgres psql -U user -d usersdb < other/backups/$(FILE) && \
			echo "✓ База успешно восстановлена из $(FILE)"; \
			\
			echo "👥 Проверьте количество пользователей: go run ./other/tools/database/editClient.go"; \
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

# docker
## Запустить с показом логгов
logs: 
	docker compose logs -f

## Остановить докер
dcd:
	docker compose down

# только app + postgres
# Если используешь Vault с AppRole — передай SecretID:
#   VAULT_SECRET_ID=xxx make dcm
dcm:
	docker compose -f docker-compose.minimal.yml up -d --build
	docker compose logs -f

# Логи минимальной конфигурации
mini-logs:
	docker compose -f docker-compose.minimal.yml logs -f

# Остановить минимальную конфигурацию
dcdm:
	docker compose -f docker-compose.minimal.yml down

server: 
	git pull
	docker compose pull
	make dcm

# Запуск dev окружения (без Go приложения)
dev: dev-stop
	docker compose -f docker-compose.dev.yml up -d --build

# Остановить dev окружение
dev-stop:
	docker compose -f docker-compose.dev.yml down

# =====
# VAULT
# =====

# Показать статус Vault
vault-status:
	@echo "📊 Статус Vault:"
	@docker exec -e VAULT_ADDR=http://127.0.0.1:8200 vault vault status 2>/dev/null || echo "  Vault не запущен"

# Инициализация Vault (только один раз при первом запуске)
# ВАЖНО: сохрани выведенные ключи и root token!
vault-init:
	@echo "⚠️  Инициализация Vault..."
	@docker exec -e VAULT_ADDR=http://127.0.0.1:8200 vault vault operator init

# Распечатать Vault (нужно 3 ключа)
vault-unseal:
	@echo "🔄 Распечатываю Vault..."
	@printf "Введите unseal key 1: "; read k1; \
	 printf "Введите unseal key 2: "; read k2; \
	 printf "Введите unseal key 3: "; read k3; \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 vault vault operator unseal $$k1 && \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 vault vault operator unseal $$k2 && \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 vault vault operator unseal $$k3

# Настроить AppRole и записать секреты из .env в Vault
# Требует VAULT_TOKEN из vault-init
vault-setup:
	@echo "🔐 Настройка AppRole..."
	 @printf "Введите root token: "; read token; \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=$$token vault vault policy write proxymaster - <<< 'path "secret/data/proxymaster/*" { capabilities = ["read", "list"] }' && \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=$$token vault vault auth enable approle && \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=$$token vault vault write auth/approle/role/proxymaster secret_id_ttl=24h token_ttl=1h token_policies=proxymaster && \
	 echo "✅ AppRole создана" && \
	 echo "" && \
	 echo "📋 RoleID:" && \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=$$token vault vault read auth/approle/role/proxymaster/role-id && \
	 echo "" && \
	 echo "📋 SecretID (нажми Enter):" && \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=$$token vault vault write -f auth/approle/role/proxymaster/secret-id

# Записать секреты из .env в Vault
vault-put-secrets:
	@echo "📝 Записываю секреты в Vault..."
	@printf "Введите root token: "; read token; \
	 args=""; \
	 while IFS='=' read -r key val; do \
	   [[ "$$key" =~ ^#.*$$ ]] && continue; \
	   [[ -z "$$key" ]] && continue; \
	   [[ "$$key" == "REMNA_PANEL" ]] && continue; \
	   [[ "$$key" == "REMNA_SQUAD_UUID_2" ]] && continue; \
	   [[ "$$key" == "PLATEGA_BASE_URL" ]] && continue; \
	   [[ "$$key" == "PLATEGA_MERCHANT_ID" ]] && continue; \
	   [[ "$$key" == "YOUKASSA_BASE_URL" ]] && continue; \
	   [[ "$$key" == VAULT_* ]] && continue; \
	   val="$${val%\"}"; val="$${val#\"}"; \
	   args="$$args $$key=$$val"; \
	 done < .env; \
	 docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=$$token vault vault kv put secret/proxymaster $$args
