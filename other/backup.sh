#!/bin/bash

# Скрипт для автоматического бекапа базы данных PostgreSQL из Docker контейнера
# Использует pg_dump для создания дампа БД и сохраняет его в папку backups/ с датой

set -e

# Определяем путь к директории, где находится этот скрипт
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="$SCRIPT_DIR/backups"
LOGS_DIR="$SCRIPT_DIR/logs"
CONTAINER_NAME="users_postgres"
DB_USER="user"
DB_NAME="usersdb"
DB_PASSWORD="userspass"
RETENTION_DAYS=7

# Создаём папки, если их ещё нет
mkdir -p "$BACKUP_DIR"
mkdir -p "$LOGS_DIR"

# Генерируем имя файла с текущей датой и временем
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
BACKUP_FILE="$BACKUP_DIR/backup_${TIMESTAMP}.sql"
LOG_FILE="$LOGS_DIR/backup.log"

# Функция логирования
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "=========================================="
log "Начинается создание бекапа БД"
log "=========================================="

# Проверяем, запущен ли контейнер
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  log "✗ ОШИБКА: контейнер $CONTAINER_NAME не запущен"
  exit 1
fi

log "✓ Контейнер найден"

# Выполняем pg_dump через Docker контейнер
# Используем PGPASSWORD для избежания интерактивного ввода пароля
if docker exec -e PGPASSWORD=$DB_PASSWORD "$CONTAINER_NAME" pg_dump \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  --no-password \
  > "$BACKUP_FILE" 2>> "$LOG_FILE"; then

  # Проверяем размер файла
  if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
    SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    LINES=$(wc -l < "$BACKUP_FILE")
    log "✓ Бекап успешно создан"
    log "  Файл: $BACKUP_FILE"
    log "  Размер: $SIZE"
    log "  Строк: $LINES"
  else
    log "✗ ОШИБКА: бекап создан, но файл пуст"
    rm -f "$BACKUP_FILE"
    exit 1
  fi
else
  log "✗ ОШИБКА: не удалось создать бекап (pg_dump вернул ошибку)"
  exit 1
fi

# Удаляем старые бекапы (старше RETENTION_DAYS дней)
log "Удаление старых бекапов (старше $RETENTION_DAYS дней)..."
OLD_BACKUPS=$(find "$BACKUP_DIR" -type f -name "backup_*.sql" -mtime +$RETENTION_DAYS 2>/dev/null | wc -l)
if [ "$OLD_BACKUPS" -gt 0 ]; then
  find "$BACKUP_DIR" -type f -name "backup_*.sql" -mtime +$RETENTION_DAYS -delete
  log "✓ Удалено старых бекапов: $OLD_BACKUPS"
else
  log "  Старых бекапов не найдено"
fi

# Показываем итоговую статистику
TOTAL_BACKUPS=$(find "$BACKUP_DIR" -type f -name "backup_*.sql" 2>/dev/null | wc -l)
TOTAL_SIZE=$(du -sh "$BACKUP_DIR" 2>/dev/null | cut -f1)

log "=========================================="
log "✓ Бекап завершен успешно"
log "  Всего бекапов: $TOTAL_BACKUPS"
log "  Общий размер: $TOTAL_SIZE"
log "=========================================="
log ""
