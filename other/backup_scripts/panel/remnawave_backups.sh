#!/bin/bash

# 1. Создаем папку для бэкапов и выставляем права
mkdir -p /root/remnawave_backups
chmod 700 /root/remnawave_backups

# 2. Создаем файл скрипта бэкапа (pg_backup_remnawave.sh)
cat <<'EOF' > /root/pg_backup_remnawave.sh
#!/bin/bash
set -e

CONTAINER="remnawave-db"
DB_NAME="postgres"
DB_USER="postgres"
DB_PASSWORD="postgres"

BACKUP_DIR="/root/remnawave_backups"
DATE=$(date +%F_%H-%M)
BACKUP_FILE="postgres_backup_${DATE}.dump"
TMP_PATH="/tmp/${BACKUP_FILE}"

echo "[INFO] Starting PostgreSQL backup: $DATE"

docker exec \
  -e PGPASSWORD="$DB_PASSWORD" \
  "$CONTAINER" \
  pg_dump \
    -U "$DB_USER" \
    -F c \
    -b \
    -v \
    -f "$TMP_PATH" \
    "$DB_NAME"

docker cp "$CONTAINER:$TMP_PATH" "$BACKUP_DIR/$BACKUP_FILE"
docker exec "$CONTAINER" rm -f "$TMP_PATH"

find "$BACKUP_DIR" -type f -name "*.dump" -mtime +1 -delete

echo "[INFO] Backup completed successfully"
EOF

# 3. Делаем скрипт бэкапа исполняемым
chmod +x /root/pg_backup_remnawave.sh

# 4. Добавляем задачу в crontab (каждый день в 03:00)
# Удаляем старую запись если есть, чтобы избежать дубликатов
(crontab -l 2>/dev/null | grep -v "pg_backup_remnawave.sh"; echo "0 3 * * * /root/pg_backup_remnawave.sh >> /root/remnawave_backups/backup.log 2>&1") | crontab -

echo "================================================"
echo "Установка завершена!"
echo "Скрипт бэкапа: /root/pg_backup_remnawave.sh"
echo "Папка бэкапов: /root/remnawave_backups"
echo "Расписание: Ежедневно в 03:00 (проверьте crontab -l)"
echo "================================================"
