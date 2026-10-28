#!/usr/bin/env bash
set -euo pipefail

# ==============================================================
#  ProxyMaster — перенос оплаченных доп. устройств из старой версии
#  в новую модель мест. Один файл, без Go и без сборки: нужны только
#  docker и psql внутри контейнеров.
#
#  Запускать из папки новой версии (там, где docker-compose.yml).
#
#    ./migrate_old_version.sh --dry-run     # только показать план
#    ./migrate_old_version.sh               # мигрировать и переключить бота
#    ./migrate_old_version.sh --no-cutover  # мигрировать, бота не трогать
#    ./migrate_old_version.sh -y            # без вопросов
#
#  Что берётся откуда:
#    старая база  — контейнер users_postgres (usersdb), только чтение;
#    панель       — контейнер remnawave-db (лимиты и сроки подписок);
#    новая база   — сервис postgres из этого compose;
#    старый бот   — контейнер proxymaster_app, останавливается перед стартом нового.
#
#  Флаги: --old=ИМЯ --panel=ИМЯ --app=ИМЯ --old-bot=ИМЯ --base-limit=N
# ==============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OLD_CONTAINER="users_postgres"
PANEL_CONTAINER="remnawave-db"
APP_CONTAINER=""
OLD_BOT_CONTAINER="proxymaster_app"
APP_SERVICE="postgres"
BOT_SERVICE="bot"

BASE_LIMIT=""
APPLY=1
CUTOVER=1
YES=0

usage() {
  cat <<'TXT'
Использование: migrate_old_version.sh [--dry-run] [--no-cutover] [-y] [флаги]

  --dry-run        показать план, ничего не записывать
  --no-cutover     записать места, но не останавливать старого бота
  -y, --yes        не задавать вопросов
  --old=...        контейнер старой базы (по умолчанию users_postgres)
  --panel=...      контейнер панели (по умолчанию remnawave-db)
  --app=...        контейнер новой базы (по умолчанию из compose)
  --old-bot=...    контейнер старого бота (по умолчанию proxymaster_app)
  --base-limit=N   базовых мест на пользователя (по умолчанию DEVICE_LIMIT или 2)
TXT
}

for arg in "$@"; do
  case "$arg" in
    --dry-run) APPLY=0; CUTOVER=0 ;;
    --no-cutover) CUTOVER=0 ;;
    -y|--yes) YES=1 ;;
    -h|--help) usage; exit 0 ;;
    --old=*) OLD_CONTAINER="${arg#*=}" ;;
    --panel=*) PANEL_CONTAINER="${arg#*=}" ;;
    --app=*) APP_CONTAINER="${arg#*=}" ;;
    --old-bot=*) OLD_BOT_CONTAINER="${arg#*=}" ;;
    --base-limit=*) BASE_LIMIT="${arg#*=}" ;;
    *) echo "Неизвестный аргумент: $arg (см. --help)" >&2; exit 1 ;;
  esac
done

log() { echo "==> $*"; }
die() { echo "ОШИБКА: $*" >&2; exit 1; }

running() { docker ps --format '{{.Names}}' | grep -qx "$1"; }

# Строки сервиса базы из compose: от его имени до следующего сервиса.
compose_service() {
  local svc="$APP_SERVICE"
  awk -v svc="$svc" '
    /^[[:space:]]*[a-z_]+:[[:space:]]*$/ {
      name=$1; gsub(/[:[:space:]]/, "", name)
      if (name == svc) { on=1; next }
      if (!on) { next }
      if ($0 !~ /^[[:space:]]/) { on=0 }
    }
    on' "$COMPOSE_DIR/docker-compose.yml" 2>/dev/null
}

# Порт, на который compose хочет посадить базу: "5432" или "127.0.0.1:5433".
# У запущенного контейнера берём фактический, иначе значение из compose.
compose_port() {
  local cid
  cid="$("${COMPOSE[@]}" ps -q "$APP_SERVICE" 2>/dev/null | head -1 || true)"
  if [[ -n "$cid" ]]; then
    docker port "$cid" 5432/tcp 2>/dev/null | head -1 | sed 's/.*://'
    return
  fi
  compose_service | awk '/^[[:space:]]+-[[:space:]]*"/ && /:5432"/ {
    line=$0; gsub(/["[:space:]]/, "", line)
    n=split(line, a, ":"); print a[n]; exit }'
}

compose_field() {
  compose_service | sed -n "s/^[[:space:]]*$1:[[:space:]]*//p" | head -1
}

compose_volume() {
  compose_service | awk '/^[[:space:]]*-/ && /\/var\/lib\/postgresql/ { print $2 }' | head -1 | sed 's/:.*//'
}

port_busy() {
  ss -ltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]$1\$" \
    || (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

# env контейнера: доступы официального образа postgres лежат в POSTGRES_*.
# У временного контейнера они перебиты на unused, поэтому берём исходные
# значения из pg_hba/initdb нельзя — читаем их из старого контейнера.
cenv() {
  local src="$1"
  [[ "$src" == *"_readonly" ]] && src="${src%_readonly}"
  docker inspect -f "{{range .Config.Env}}{{println .}}{{end}}" "$src" 2>/dev/null \
    | sed -n "s/^$2=//p" | head -1
}

psql_in() {
  local c="$1" u="$2" d="$3"
  shift 3
  docker exec -i "$c" psql -U "$u" -d "$d" -X -q "$@"
}

wait_db() {
  local c="$1" u="$2" d="$3" i
  for i in $(seq 1 60); do
    if docker exec "$c" pg_isready -U "$u" -d "$d" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  die "база в $c не поднялась за 60 секунд"
}

# --- 0. где compose новой версии ---

DB_STANDALONE=0
COMPOSE_DIR=""
for d in "$PWD" "$SCRIPT_DIR" "$SCRIPT_DIR/.." "$SCRIPT_DIR/../.."; do
  d="$(cd "$d" 2>/dev/null && pwd)" || continue
  # compose старой версии тоже содержит сервис postgres: он не подходит.
  if [[ -f "$d/docker-compose.yml" ]] \
     && grep -qE '^[[:space:]]+postgres:' "$d/docker-compose.yml" \
     && grep -qE '^[[:space:]]+bot:' "$d/docker-compose.yml"; then
    COMPOSE_DIR="$d"
    break
  fi
done

COMPOSE=()
if [[ -n "$COMPOSE_DIR" ]]; then
  COMPOSE=(docker compose -f "$COMPOSE_DIR/docker-compose.yml")
fi

if [[ -z "$APP_CONTAINER" && ${#COMPOSE[@]} -eq 0 ]]; then
  die "не нашёл docker-compose.yml новой версии рядом с $PWD
Положите скрипт в папку новой версии или укажите базу флагом --app=ИМЯ"
fi

if [[ "$CUTOVER" == "1" && ${#COMPOSE[@]} -eq 0 ]]; then
  die "для переключения бота нужен docker-compose.yml новой версии (или --no-cutover)"
fi

if [[ "$BASE_LIMIT" == "" ]]; then
  BASE_LIMIT="2"
  if [[ -n "$COMPOSE_DIR" && -f "$COMPOSE_DIR/.env" ]]; then
    v="$(grep -E '^DEVICE_LIMIT=' "$COMPOSE_DIR/.env" | tail -1 | cut -d= -f2- | xargs || true)"
    v="${v%%[[:space:]#]*}"
    # Мусор в DEVICE_LIMIT молча не подменяем: иначе лимиты уедут в другие места.
    if [[ -n "${v:-}" ]]; then
      [[ "$v" =~ ^[0-9]+$ ]] && [[ "$v" -gt 0 ]] || die "DEVICE_LIMIT в .env не число: '$v'"
      BASE_LIMIT="$v"
    fi
    unset v
  fi
fi

# --- 0b. хватает ли настроек новому боту ---

if [[ -n "$COMPOSE_DIR" && ! -f "$COMPOSE_DIR/.env" ]]; then
  if [[ -f "$COMPOSE_DIR/../ProxyMaster_v2/.env" ]]; then
    cp "$COMPOSE_DIR/../ProxyMaster_v2/.env" "$COMPOSE_DIR/.env"
    chmod 600 "$COMPOSE_DIR/.env"
    log "взял .env у старой версии: $COMPOSE_DIR/../ProxyMaster_v2/.env"
  else
    die "нет $COMPOSE_DIR/.env: скопируйте туда .env старой версии"
  fi
fi

if [[ -n "$COMPOSE_DIR" && -f "$COMPOSE_DIR/.env" ]]; then
  MISSING=()
  for k in TELEGRAM_TOKEN TELEGRAM_SUPPORT REMNA_PANEL REMNA_TOKEN REMNA_SQUAD_UUID PAYMENT_PROVIDER; do
    grep -qE "^$k=.+" "$COMPOSE_DIR/.env" || MISSING+=("$k")
  done
  if [[ ${#MISSING[@]} -gt 0 ]]; then
    die "в $COMPOSE_DIR/.env нет обязательных настроек: ${MISSING[*]}"
  fi
fi

# --- 1. что запущено ---

running "$PANEL_CONTAINER" || die "контейнер панели $PANEL_CONTAINER не запущен"

# Старую базу читаем, пока она запущена. Если её уже остановили (освобождая
# порт 5432), читаем с того же тома через временный контейнер: данные от этого
# не меняются, а вернуть старую базу на место можно в любой момент.
OLD_TEMP=0
if ! running "$OLD_CONTAINER"; then
  OLD_IMAGE="$(docker inspect -f '{{.Config.Image}}' "$OLD_CONTAINER" 2>/dev/null || true)"
  OLD_VOLUME="$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/var/lib/postgresql/data"}}{{.Name}}{{end}}{{end}}' "$OLD_CONTAINER" 2>/dev/null || true)"
  if [[ -z "$OLD_IMAGE" || -z "$OLD_VOLUME" ]]; then
    die "контейнер старой базы $OLD_CONTAINER не запущен и его том не найден"
  fi

  docker rm -f "${OLD_CONTAINER}_readonly" >/dev/null 2>&1 || true
  docker run -d --name "${OLD_CONTAINER}_readonly" \
    -e POSTGRES_USER=unused -e POSTGRES_PASSWORD=unused \
    -v "$OLD_VOLUME:/var/lib/postgresql/data" "$OLD_IMAGE" >/dev/null
  OLD_CONTAINER="${OLD_CONTAINER}_readonly"
  OLD_TEMP=1
fi

readonly_cleanup() {
  if [[ "$OLD_TEMP" == "1" ]]; then
    docker rm -f "$OLD_CONTAINER" >/dev/null 2>&1 || true
  fi
}
trap 'readonly_cleanup; cleanup' EXIT

if [[ -n "$APP_CONTAINER" ]]; then
  running "$APP_CONTAINER" || die "контейнер новой базы $APP_CONTAINER не запущен"
else
  APP_CONTAINER="$("${COMPOSE[@]}" ps -q "$APP_SERVICE" 2>/dev/null || true)"
  if [[ -z "$APP_CONTAINER" ]]; then
    # compose публикует базу на 5432, а там уже старая версия. Чужой compose
    # не правим: поднимаем базу напрямую с теми же env и томом, что в compose,
    # только на свободном порту.
    WANT_PORT="$(compose_port)"
    [[ -n "$WANT_PORT" ]] || WANT_PORT=5432
    PUB_PORT="$WANT_PORT"
    if port_busy "$WANT_PORT"; then
      PUB_PORT=""
      for p in 5433 5434 5435 5440 5442; do
        port_busy "$p" || { PUB_PORT="$p"; break; }
      done
      [[ -n "$PUB_PORT" ]] || die "нет свободного порта рядом с $WANT_PORT для новой базы"

      DB_IMAGE="$(compose_field image)"
      DB_VOLUME="$(compose_volume)"
      [[ -n "$DB_IMAGE" ]] || die "не нашёл image для $APP_SERVICE в $COMPOSE_DIR/docker-compose.yml"
      [[ -n "$DB_VOLUME" ]] || die "не нашёл volume данных для $APP_SERVICE в $COMPOSE_DIR/docker-compose.yml"

      APP_NAME="${COMPOSE_DIR##*/}_$APP_SERVICE"
      DB_PASS="change_me"
      if [[ -f "$COMPOSE_DIR/.env" ]]; then
        v="$(grep -E '^POSTGRES_PASSWORD=' "$COMPOSE_DIR/.env" | tail -1 | cut -d= -f2- | xargs || true)"
        v="${v%%[[:space:]#]*}"
        [[ -n "${v:-}" ]] && DB_PASS="$v"
        unset v
      fi

      log "порт $WANT_PORT занят (старая версия): поднимаю базу на $PUB_PORT как $APP_NAME"
      docker run -d --name "$APP_NAME" --restart unless-stopped \
        -e POSTGRES_DB=app -e POSTGRES_USER=app -e POSTGRES_PASSWORD="$DB_PASS" \
        -p "127.0.0.1:$PUB_PORT:5432" -v "$DB_VOLUME:/var/lib/postgresql" \
        "$DB_IMAGE" >/dev/null
      APP_CONTAINER="$APP_NAME"
      DB_STANDALONE=1
    else
      log "поднимаю новую базу (${COMPOSE_DIR##*/}/$APP_SERVICE)"
      "${COMPOSE[@]}" up -d "$APP_SERVICE"
      APP_CONTAINER="$("${COMPOSE[@]}" ps -q "$APP_SERVICE")"
    fi
  fi
  [[ -n "$APP_CONTAINER" ]] || die "не удалось поднять $APP_SERVICE из $COMPOSE_DIR/docker-compose.yml"
fi

APP_NAME="$(docker inspect -f '{{.Name}}' "$APP_CONTAINER" | sed 's#^/##')"

OLD_USER="$(cenv "$OLD_CONTAINER" POSTGRES_USER)"; OLD_USER="${OLD_USER:-user}"
OLD_DB="$(cenv "$OLD_CONTAINER" POSTGRES_DB)"; OLD_DB="${OLD_DB:-usersdb}"
PANEL_USER="$(cenv "$PANEL_CONTAINER" POSTGRES_USER)"; PANEL_USER="${PANEL_USER:-postgres}"
PANEL_DB="$(cenv "$PANEL_CONTAINER" POSTGRES_DB)"; PANEL_DB="${PANEL_DB:-postgres}"
APP_USER="$(cenv "$APP_NAME" POSTGRES_USER)"; APP_USER="${APP_USER:-app}"
APP_DB="$(cenv "$APP_NAME" POSTGRES_DB)"; APP_DB="${APP_DB:-app}"

wait_db "$APP_NAME" "$APP_USER" "$APP_DB"

OLD_TZ="$(psql_in "$OLD_CONTAINER" "$OLD_USER" "$OLD_DB" -At -c "show timezone" | tr -d '[:space:]')"
[[ -n "$OLD_TZ" ]] || die "не удалось прочитать часовой пояс старой базы"

log "источники:"
echo "    старая база: $OLD_CONTAINER ($OLD_DB), пояс $OLD_TZ"
echo "    панель:      $PANEL_CONTAINER ($PANEL_DB)"
echo "    новая база:  $APP_NAME ($APP_DB)"

# --- 2. снимок источников в UTC ---

TMPD="$(mktemp -d "${TMPDIR:-/tmp}/pm-migrate.XXXXXX")"
CONTAINER_FILES=0

cleanup() {
  if [[ "$CONTAINER_FILES" == "1" ]]; then
    docker exec "$APP_NAME" rm -f /tmp/pm_migrate_addons.tsv /tmp/pm_migrate_panel.tsv /tmp/pm_migrate.sql 2>/dev/null || true
  fi
  rm -rf "$TMPD"
}
trap 'readonly_cleanup; cleanup' EXIT

# Времена старых таблиц без пояса: переводим в абсолютный момент по поясу
# самой базы и отдаём в UTC, чтобы новая база не гадала.
ADDONS_SQL=$(cat <<'SQL'
SET TIME ZONE 'UTC';
SELECT id, user_id,
       to_char((next_charge_at AT TIME ZONE :'tz') AT TIME ZONE 'UTC',
               'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
       to_char((COALESCE(created_at, next_charge_at) AT TIME ZONE :'tz') AT TIME ZONE 'UTC',
               'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
FROM device_addons
WHERE active
ORDER BY user_id, next_charge_at, id;
SQL
)
psql_in "$OLD_CONTAINER" "$OLD_USER" "$OLD_DB" -v tz="$OLD_TZ" -At -F $'\t' \
  <<< "$ADDONS_SQL" > "$TMPD/addons.tsv"

# Панель хранит expire_at как timestamp в UTC.
PANEL_SQL=$(cat <<'SQL'
SET TIME ZONE 'UTC';
SELECT username,
       to_char((expire_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC',
               'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
       COALESCE(hwid_device_limit::text, '\N')
FROM users
WHERE username ~ '^[0-9]+$'
ORDER BY username;
SQL
)
psql_in "$PANEL_CONTAINER" "$PANEL_USER" "$PANEL_DB" -At -F $'\t' \
  <<< "$PANEL_SQL" > "$TMPD/panel.tsv"

echo "    активных строк доп. устройств: $(wc -l < "$TMPD/addons.tsv" | tr -d ' ')"
echo "    пользователей панели:          $(wc -l < "$TMPD/panel.tsv" | tr -d ' ')"

# --- 3. план и запись ---

NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

cat > "$TMPD/migrate.sql" <<'SQL'
\set ON_ERROR_STOP on
SET TIME ZONE 'UTC';
SET client_min_messages = warning;

BEGIN;

-- Схема новая, но бот мог её ещё не создать: держим в синхроне
-- с internal/platform/postgres/postgres.go.
CREATE TABLE IF NOT EXISTS devices (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username   TEXT        NOT NULL,
    hwid       TEXT,
    label      TEXT,
    kind       TEXT        NOT NULL DEFAULT 'base',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'base';
CREATE UNIQUE INDEX IF NOT EXISTS devices_username_hwid_uq
    ON devices (username, hwid) WHERE hwid IS NOT NULL;
CREATE INDEX IF NOT EXISTS devices_username_idx ON devices (username);
CREATE INDEX IF NOT EXISTS devices_to_revoke_idx
    ON devices (expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS devices_extra_to_revoke_idx
    ON devices (expires_at) WHERE kind = 'extra' AND revoked_at IS NULL;
CREATE TABLE IF NOT EXISTS processed_payments (
    payment_id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TEMP TABLE src_addon (
    id text, username text, paid_until timestamptz, created_at timestamptz
) ON COMMIT DROP;
\copy src_addon FROM '/tmp/pm_migrate_addons.tsv'

CREATE TEMP TABLE panel_user (
    username text, expire_at timestamptz, hwid_limit int
) ON COMMIT DROP;
\copy panel_user FROM '/tmp/pm_migrate_panel.tsv'

-- План: оплаченное место на каждый активный доп с будущей датой и добивка
-- лимита панели до прежнего значения за вычетом базовых и оплаченных.
CREATE TEMP TABLE plan_seats ON COMMIT DROP AS
SELECT 'legacy:device_addon:' || a.id AS marker,
       a.username,
       a.paid_until                    AS expires_at,
       a.created_at                    AS created_at,
       'paid'::text                    AS origin
FROM src_addon a
WHERE a.paid_until > :'now'::timestamptz

UNION ALL

SELECT 'legacy:panel_limit:' || u.username || ':' || g.n,
       u.username,
       u.expire_at,
       :'now'::timestamptz,
       'limit'
FROM panel_user u
LEFT JOIN (
    SELECT username, count(*) AS n
    FROM src_addon
    WHERE paid_until > :'now'::timestamptz
    GROUP BY username
) p ON p.username = u.username
CROSS JOIN LATERAL generate_series(
    1,
    GREATEST(COALESCE(u.hwid_limit, 0) - :base - COALESCE(p.n, 0), 0)
) AS g(n)
WHERE u.hwid_limit IS NOT NULL
  AND u.expire_at > :'now'::timestamptz;

\echo ''
\echo 'План:'
SELECT CASE origin WHEN 'paid' THEN 'оплаченные' ELSE 'из лимита панели' END AS "источник",
       count(*)                     AS "мест",
       count(DISTINCT username)     AS "пользователей"
FROM plan_seats
GROUP BY origin
ORDER BY origin;

\echo ''
\echo 'По пользователям:'
SELECT s.username                                        AS "username",
       COALESCE(p.hwid_limit::text, 'NULL')              AS "лимит",
       count(*) FILTER (WHERE s.origin = 'paid')         AS "оплачено",
       count(*) FILTER (WHERE s.origin = 'limit')        AS "из лимита",
       COALESCE(to_char(p.expire_at, 'YYYY-MM-DD'), '-') AS "подписка до"
FROM plan_seats s
LEFT JOIN panel_user p ON p.username = s.username
GROUP BY s.username, p.hwid_limit, p.expire_at
ORDER BY s.username;

\echo ''
\echo 'Предупреждения:'
SELECT '  ! ' || u.username || ': оплата есть, активной подписки в панели нет' AS "предупреждение"
FROM panel_user u
RIGHT JOIN (SELECT DISTINCT username FROM plan_seats WHERE origin = 'paid') pa
       ON pa.username = u.username
WHERE u.username IS NULL OR u.expire_at <= :'now'::timestamptz;

SELECT '  ! ' || u.username || ': лимит ' || u.hwid_limit ||
       ' меньше, чем ' || :base || ' базовых + ' || COALESCE(p.n, 0) || ' оплаченных' AS "предупреждение"
FROM panel_user u
LEFT JOIN (
    SELECT username, count(*) AS n FROM src_addon
    WHERE paid_until > :'now'::timestamptz GROUP BY username
) p ON p.username = u.username
WHERE u.hwid_limit IS NOT NULL
  AND u.hwid_limit > 0
  AND u.expire_at > :'now'::timestamptz
  AND u.hwid_limit - COALESCE(p.n, 0) < :base;

SELECT '  ! ' || string_agg(username, ', ' ORDER BY username) ||
       ': подписка истекла, лимит больше базового — места из лимита не переносятся' AS "предупреждение"
FROM panel_user
WHERE expire_at <= :'now'::timestamptz
  AND hwid_limit IS NOT NULL
  AND hwid_limit > :base
HAVING count(*) > 0;

\if :apply

-- Маркер и место пишутся одним стейтментом: если место не вставилось,
-- маркер тоже откатится, и повторный запуск снова его возьмёт.
WITH claimed AS (
    INSERT INTO processed_payments (payment_id)
    SELECT marker FROM plan_seats
    ON CONFLICT (payment_id) DO NOTHING
    RETURNING payment_id
), ins AS (
    INSERT INTO devices (username, kind, expires_at, created_at)
    SELECT s.username, 'extra', s.expires_at, s.created_at
    FROM plan_seats s
    JOIN claimed c ON c.payment_id = s.marker
    RETURNING 1
)
SELECT (SELECT count(*) FROM claimed) AS "маркеров занято",
       (SELECT count(*) FROM ins)     AS "мест записано";

-- Сверка: для каждой пары (пользователь, срок) мест должно быть не меньше,
-- чем в плане. Учитываем и места, записанные прошлыми запусками.
DO $$
DECLARE missing int;
BEGIN
    SELECT count(*) INTO missing
    FROM (
        SELECT username, expires_at, count(*) AS need
        FROM plan_seats GROUP BY 1, 2
    ) n
    LEFT JOIN (
        SELECT username, expires_at, count(*) AS have
        FROM devices
        WHERE kind = 'extra' AND revoked_at IS NULL
        GROUP BY 1, 2
    ) h USING (username, expires_at)
    WHERE COALESCE(h.have, 0) < n.need;

    IF missing > 0 THEN
        RAISE EXCEPTION 'сверка не сошлась: не хватает % позиций плана', missing;
    END IF;
END $$;

COMMIT;

\echo ''
\echo 'Состояние новой базы:'
SELECT count(*)                                  AS "доп. мест",
       count(DISTINCT username)                  AS "пользователей",
       count(*) FILTER (WHERE expires_at < now()) AS "уже истекли"
FROM devices
WHERE kind = 'extra' AND revoked_at IS NULL;

\else

ROLLBACK;
\echo ''
\echo 'Это был пробный прогон: в новую базу ничего не записано.'

\endif
SQL

docker cp "$TMPD/addons.tsv"   "$APP_NAME:/tmp/pm_migrate_addons.tsv" >/dev/null
docker cp "$TMPD/panel.tsv"    "$APP_NAME:/tmp/pm_migrate_panel.tsv"  >/dev/null
docker cp "$TMPD/migrate.sql"  "$APP_NAME:/tmp/pm_migrate.sql"        >/dev/null
CONTAINER_FILES=1

log "считаю план и пишу места ($APP_DB)"
psql_in "$APP_NAME" "$APP_USER" "$APP_DB" \
  -v apply="$APPLY" -v base="$BASE_LIMIT" -v now="$NOW" \
  -f /tmp/pm_migrate.sql

if [[ "$APPLY" != "1" ]]; then
  echo
  echo "Пробный прогон закончен. Для миграции запустите без --dry-run."
  exit 0
fi

# --- 4. переключение бота ---

if [[ "$CUTOVER" != "1" ]]; then
  echo
  echo "Места записаны, бот не переключён (--no-cutover)."
  echo "Когда будете готовы: docker stop $OLD_BOT_CONTAINER && docker compose up -d --build $BOT_SERVICE"
  exit 0
fi

echo
if [[ "$YES" != "1" ]]; then
  CONFIRM="n"
  read -r -p "Остановить старого бота и поднять нового? (y/N): " CONFIRM || true
  if [[ "${CONFIRM:-n}" != "y" && "${CONFIRM:-n}" != "Y" ]]; then
    echo "Миграция записана, переключение отменено."
    exit 0
  fi
fi

rollback() {
  echo "!!! новый бот не поднялся, возвращаю старого" >&2
  "${COMPOSE[@]}" stop "$BOT_SERVICE" >/dev/null 2>&1 || true
  docker start "$OLD_BOT_CONTAINER" >/dev/null 2>&1 || true
}

log "собираю образ нового бота (до остановки старого, чтобы не растягивать простой)"
"${COMPOSE[@]}" build "$BOT_SERVICE" || die "сборка не прошла, старый бот не тронут"

# База, поднятая docker run, compose не видит: он попытался бы поднять вторую
# на том же томе. Поэтому поднимаем бота без зависимостей и сами говорим,
# где база — по имени контейнера.
BOT_UP=(up -d)
BOT_EXTRA=()
if [[ "$DB_STANDALONE" == "1" ]]; then
  BOT_UP=(up -d --no-deps)
  BOT_EXTRA=(-e "DATABASE_URL=postgres://app:$APP_PASS@$APP_NAME:5432/app?sslmode=disable")
fi

if running "$OLD_BOT_CONTAINER"; then
  log "останавливаю старого бота $OLD_BOT_CONTAINER"
  docker stop "$OLD_BOT_CONTAINER" >/dev/null
else
  echo "    старый бот $OLD_BOT_CONTAINER не запущен, пропускаю"
fi

log "поднимаю нового бота"
if ! "${COMPOSE[@]}" "${BOT_UP[@]}" "${BOT_EXTRA[@]}" "$BOT_SERVICE"; then
  rollback

  die "compose не смог поднять $BOT_SERVICE, старый бот возвращён"
fi

BOT_CID="$("${COMPOSE[@]}" ps -q "$BOT_SERVICE")"
[[ -n "$BOT_CID" ]] || die "новый бот не появился в compose"

for i in $(seq 1 60); do
  [[ "$(docker inspect -f '{{.State.Status}}' "$BOT_CID")" == "running" ]] && break
  sleep 1
done

sleep 5

if [[ "$(docker inspect -f '{{.State.Status}}' "$BOT_CID")" != "running" ]]; then
  docker logs "$BOT_CID" 2>&1 | tail -20
  rollback

  die "новый бот не запустился, смотрите логи: docker logs ${BOT_CID:0:12}"
fi

if docker logs "$BOT_CID" 2>&1 | tail -50 | grep -qE 'panic|FATAL'; then
  docker logs "$BOT_CID" 2>&1 | tail -20
  rollback

  die "новый бот пишет panic/FATAL в логах"
fi

log "новый бот запущен: $BOT_SERVICE"
docker logs "$BOT_CID" 2>&1 | tail -10

echo
echo "Проверьте в Telegram у клиента с оплаченными допами: число мест и сроки."
echo "Откат: docker stop \$(docker compose ps -q $BOT_SERVICE) && docker start $OLD_BOT_CONTAINER"
