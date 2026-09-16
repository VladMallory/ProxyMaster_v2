#!/usr/bin/env bash
set -euo pipefail

# ==============================================================
#  Remnawave — сброс админа панели 
#  Прям стирает всех админов из БД, чтобы панель предложила
#  создать нового суперадмина с нуля.
#  (аналог Rescue CLI -> Reset superadmin, но без TTY)
#
#  Использование:
#    ./reset-admin.sh           # с подтверждением
#    ./reset-admin.sh -y        # без подтверждения
#    ./reset-admin.sh --yes
# ==============================================================

DB_CONTAINER="remnawave-db"
REDIS_CONTAINER="remnawave-redis"
REDIS_SOCK="/var/run/valkey/valkey.sock"
DB_USER="postgres"
DB_NAME="postgres"
BACKUP_DIR="/root"
YES=0

for arg in "$@"; do
  case "$arg" in
    -y|--yes) YES=1 ;;
    -h|--help)
      echo "Использование: $0 [-y|--yes]"
      echo "  Стирает таблицу admin в БД remnawave, панель предложит создать нового суперадмина."
      exit 0
      ;;
    *) echo "Неизвестный аргумент: $arg (см. --help)" >&2; exit 1 ;;
  esac
done

# --- 0. подтянуть POSTGRES_* из /opt/remnawave/.env, если есть ---
# (.env целиком source'ить нельзя — там есть значения вроде [60, 80], которые bash пытается выполнить)
if [[ -f /opt/remnawave/.env ]]; then
  val="$(grep -E '^POSTGRES_USER=' /opt/remnawave/.env | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" | xargs)"
  [[ -n "${val:-}" ]] && DB_USER="$val"
  val="$(grep -E '^POSTGRES_DB=' /opt/remnawave/.env | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" | xargs)"
  [[ -n "${val:-}" ]] && DB_NAME="$val"
  unset val
fi

echo "==> Проверка контейнеров ..."
docker ps --format '{{.Names}}' | grep -qx "$DB_CONTAINER" || { echo "ОШИБКА: контейнер $DB_CONTAINER не запущен" >&2; exit 1; }

echo "==> Текущие админы в БД:"
docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c 'SELECT uuid, username, role, created_at FROM "admin";'

COUNT="$(docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -A -c 'SELECT count(*) FROM "admin";' | tr -d '[:space:]')"
echo "    Всего админов: $COUNT"

if [[ "$COUNT" == "0" ]]; then
  echo "==> Админов уже нет — панель уже должна предлагать создать нового. Ничего делать не нужно."
  exit 0
fi

if [[ "$YES" != "1" ]]; then
  echo ""
  echo "ВНИМАНИЕ: сейчас будут УДАЛЕНЫ ВСЕ админы ($COUNT шт), логин будет невозможен,"
  echo "а панель предложит создать нового суперадмина с нуля."
  read -r -p "Точно стереть админов? (y/N): " CONFIRM
  if [[ "${CONFIRM:-n}" != "y" && "${CONFIRM:-n}" != "Y" ]]; then
    echo "Отменено."
    exit 0
  fi
fi

# --- 1. страховочный бекап таблицы admin (+ passkeys, у них FK на admin) ---
STAMP="$(date +%Y%m%d_%H%M%S)"
BACKUP_FILE="$BACKUP_DIR/admin_backup_$STAMP.dump"
echo "==> Страховочный бекап admin/passkeys -> $BACKUP_FILE"
docker exec "$DB_CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -F c -t 'public."admin"' -t public.passkeys -f /tmp/admin_backup.dump
docker cp "$DB_CONTAINER:/tmp/admin_backup.dump" "$BACKUP_FILE"
docker exec "$DB_CONTAINER" rm -f /tmp/admin_backup.dump
echo "    Бекап сохранён: $BACKUP_FILE"

# --- 2. удаление админов (passkeys удалятся каскадом по FK) ---
echo "==> Удаляю всех админов ..."
docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c 'DELETE FROM "admin";'

# --- 3. чистка кеша настроек/сессий в redis (как делает Rescue CLI) ---
echo "==> Чищу кеш панели в redis ..."
for db in 1 0; do
  docker exec "$REDIS_CONTAINER" valkey-cli -s "$REDIS_SOCK" -n "$db" DEL "rmnwv:remnawave_settings" >/dev/null 2>&1 || true
  docker exec "$REDIS_CONTAINER" valkey-cli -s "$REDIS_SOCK" -n "$db" DEL "ioraw:remnawave_settings" >/dev/null 2>&1 || true
done

# --- 4. проверка ---
COUNT_AFTER="$(docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -A -c 'SELECT count(*) FROM "admin";' | tr -d '[:space:]')"
echo "    Админов после сброса: $COUNT_AFTER"

if [[ "$COUNT_AFTER" != "0" ]]; then
  echo "ОШИБКА: в таблице admin что-то осталось, проверь вручную." >&2
  exit 1
fi

echo ""
echo "==> Готово. Админы стёрты."
echo "    Открой панель в браузере — она предложит создать нового суперадмина с нуля."
echo "    Бекап прежних админов: $BACKUP_FILE"

