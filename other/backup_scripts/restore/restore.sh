#!/usr/bin/env bash
set -euo pipefail

# ==============================================================
#  Remnawave — восстановление из бекапа
#  Берёт самый свежий *.dump в /root и восстанавливает его в БД
# ==============================================================

DB_CONTAINER="remnawave-db"
APP_CONTAINERS=("remnawave" "remnawave-subscription-page")
BACKUP_DIR="/root"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-postgres}"

echo "==> Поиск свежего дампа в $BACKUP_DIR ..."

DUMP_FILE="$(ls -1t "$BACKUP_DIR"/*.dump 2>/dev/null | grep -v '^.*pre_restore_safety.*' | head -1)"
if [[ -z "$DUMP_FILE" ]]; then
  echo "ОШИБКА: не найден ни один *.dump в $BACKUP_DIR"
  exit 1
fi
DUMP_FILE="$(realpath "$DUMP_FILE")"
echo "    Найден дамп: $DUMP_FILE"

read -r -p "Восстановить этот дамп в БД панели? (y/N): " CONFIRM
if [[ "${CONFIRM:-n}" != "y" && "${CONFIRM:-n}" != "Y" ]]; then
  echo "Отменено."
  exit 0
fi

# --- 1. управление панелью ---
start_apps() {
  for c in "${APP_CONTAINERS[@]}"; do
    docker start "$c" >/dev/null 2>&1 || true
  done
}
stop_apps() {
  for c in "${APP_CONTAINERS[@]}"; do
    docker stop "$c" >/dev/null 2>&1 || true
  done
}
trap start_apps EXIT

# --- 2. страховочный дамп текущего состояния ---
SAFETY="/root/pre_restore_safety_$(date +%Y%m%d_%H%M%S).dump"
echo "==> Страховочный дамп текущего состояния -> $SAFETY"
docker exec "$DB_CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -F c -f /tmp/safety.dump
docker cp "$DB_CONTAINER:/tmp/safety.dump" "$SAFETY"

# --- 3. остановка панели ---
echo "==> Останавливаю панель ..."
stop_apps
sleep 3

# --- 4. копируем дамп в контейнер ---
echo "==> Копирую дамп в контейнер $DB_CONTAINER ..."
docker cp "$DUMP_FILE" "$DB_CONTAINER:/tmp/restore.dump"

# --- 5. сброс схемы ---
echo "==> Сбрасываю схему public ..."
docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" \
  -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO public; GRANT ALL ON SCHEMA public TO $DB_USER;"

# --- 6. восстановление дампа ---
echo "==> Восстанавливаю дамп ..."
docker exec "$DB_CONTAINER" pg_restore -U "$DB_USER" -d "$DB_NAME" --no-owner /tmp/restore.dump
echo "    Восстановление завершено."

# --- 7. запуск панели ---
echo "==> Запускаю панель ..."
start_apps

echo "==> Готово. Дамп $DUMP_FILE восстановлен."
echo "    Страховочный дамп прежнего состояния: $SAFETY" 
