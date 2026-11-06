#!/usr/bin/env bash
# ==============================================================
#  ProxyMaster: обновление старого бота (v1) до текущего main
#  с обязательными бэкапами и переносом мест устройств.
#
#  Запускать НА СЕРВЕРЕ от root. Сначала проверьте план:
#    ./update_to_v2.sh --app-dir /root/proxy --dry-run
#  Затем боевой запуск:
#    ./update_to_v2.sh --app-dir /root/proxy --yes
#
#  Старые контейнеры и тома не удаляются: старый бот и БД только
#  останавливаются с restart=no, чтобы откат был возможен.
# ==============================================================
set -Eeuo pipefail
umask 077

APP_DIR="/root/proxy"
OLD_BOT="proxymaster_app"
OLD_DB="users_postgres"
PANEL="remnawave-db"
PROJECT="proxymaster_v2"
PROJECT_SET=0
BRANCH="main"
BACKUP_ROOT="/root/backups"
DOMAIN=""
ACME_EMAIL=""
PRICE_OVERRIDE=""
DAYS_OVERRIDE=""
WEB_PORT_OVERRIDE=""
PUBLIC_WEB=0
DO_MIGRATE=1
DRY_RUN=0
ASSUME_YES=0
FORCE=0

TS="$(date +%Y-%m-%d-%H%M%S)"
BK=""
ENV_FILE=""
OLD_STOPPED=0
NEW_BOT_STARTED=0
NEW_VOLUME_CREATED=0
ROLLED_BACK=0
OLD_BOT_IMAGE=""
OLD_BOT_PROJECT=""
OLD_BOT_NETWORK=""
OLD_BOT_RESTART=""
OLD_DB_IMAGE=""
OLD_DB_PROJECT=""
OLD_DB_VOLUME=""
OLD_DB_MOUNT_TYPE=""
OLD_DB_MOUNT_DEST=""
OLD_DB_MOUNT_SOURCE=""
OLD_DB_USER=""
OLD_DB_NAME=""
OLD_DB_HOST="postgres"
OLD_DB_RESTART=""
TEMP_DB_VOLUME=""
DOCKER_ROOT=""
HARD_CONFLICTS=()
SOFT_CONFLICTS=()

log()  { printf '\n==> %s\n' "$*"; }
warn() { printf 'ПРЕДУПРЕЖДЕНИЕ: %s\n' "$*" >&2; }
die()  { printf 'ОШИБКА: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }
running() { docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$1"; }
device_count() { docker exec postgres psql -U app -d app -X -At -c 'select count(*) from devices' 2>/dev/null || echo "?"; }

usage() {
  cat <<'TXT'
Использование: update_to_v2.sh [опции]

  --app-dir DIR        каталог старого репозитория (по умолчанию /root/proxy)
  --old-bot NAME       контейнер старого бота (proxymaster_app)
  --old-db NAME        контейнер старой базы (users_postgres)
  --panel NAME         контейнер Remnawave (remnawave-db)
  --project NAME       имя нового compose-проекта (proxymaster_v2; если совпадает
                       со старым стеком, автоматически добавится _up)
  --branch NAME        ветка для обновления (main)
  --backup-root DIR    куда писать бэкапы (/root/backups)
  --domain DOMAIN      включить автоподключение и подготовить Traefik
                       (запуск Traefik — после настройки nginx на 80/444)
  --acme-email EMAIL   email для Let's Encrypt (нужен с --domain)
  --price N            установить PRICE_PER_MONTH
  --days 7,30,60,90    установить INITIAL_PRICE_DAYS1..4
  --web-port PORT      локальный порт web-страницы (по умолчанию 8081)
  --public-web         слушать web-порт на 0.0.0.0 (по умолчанию только localhost)
  --no-migrate         не переносить устройства, только обновить код
  --dry-run            только показать план, ничего не менять
  --yes                без интерактивных подтверждений
  --force              продолжить при мягком конфликте (остановленный стек
                       того же проекта). Конфликты имён и портов не обходятся
  -h, --help           эта справка
TXT
}

while (($#)); do
  case "$1" in
    --app-dir)       APP_DIR="${2:?}"; shift 2 ;;
    --old-bot)       OLD_BOT="${2:?}"; shift 2 ;;
    --old-db)        OLD_DB="${2:?}"; shift 2 ;;
    --panel)         PANEL="${2:?}"; shift 2 ;;
    --project)       PROJECT="${2:?}"; PROJECT_SET=1; shift 2 ;;
    --branch)        BRANCH="${2:?}"; shift 2 ;;
    --backup-root)   BACKUP_ROOT="${2:?}"; shift 2 ;;
    --domain)        DOMAIN="${2:?}"; shift 2 ;;
    --acme-email)    ACME_EMAIL="${2:?}"; shift 2 ;;
    --price)         PRICE_OVERRIDE="${2:?}"; shift 2 ;;
    --days)          DAYS_OVERRIDE="${2:?}"; shift 2 ;;
    --web-port)      WEB_PORT_OVERRIDE="${2:?}"; shift 2 ;;
    --public-web)    PUBLIC_WEB=1; shift ;;
    --no-migrate)    DO_MIGRATE=0; shift ;;
    --dry-run)       DRY_RUN=1; shift ;;
    --yes)           ASSUME_YES=1; shift ;;
    --force)         FORCE=1; shift ;;
    -h|--help)       usage; exit 0 ;;
    *) die "неизвестный аргумент: $1 (см. --help)" ;;
  esac
done

[[ $EUID -eq 0 ]] || die "скрипт нужно запускать от root"
have docker || die "не найден docker"
docker compose version >/dev/null 2>&1 || die "не найден docker compose (v2)"
[[ -d "$APP_DIR" ]] || die "нет каталога $APP_DIR"
APP_DIR="$(cd "$APP_DIR" && pwd)"
ENV_FILE="$APP_DIR/.env"
[[ -f "$ENV_FILE" ]] || die "нет $ENV_FILE"
[[ -f "$APP_DIR/docker-compose.yml" ]] || die "нет $APP_DIR/docker-compose.yml"
git -C "$APP_DIR" rev-parse --verify HEAD >/dev/null 2>&1 \
  || die "$APP_DIR не git-репозиторий (или в нём нет коммитов)"
git -C "$APP_DIR" remote get-url origin >/dev/null 2>&1 \
  || die "в $APP_DIR нет remote origin"
[[ "$OLD_DB" != "postgres" ]] \
  || die "старая БД уже называется postgres — новый compose не сможет создать свою. Переименуйте старый контейнер или обновите вручную"
[[ "$OLD_BOT" != "proxymaster_bot" ]] \
  || die "старый бот уже называется proxymaster_bot — обновление вручную"

container_env() {
  docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$1" 2>/dev/null \
    | sed -n "s/^$2=//p" | head -1
}
project_of() {
  docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$1" 2>/dev/null || true
}

docker inspect "$OLD_BOT" >/dev/null 2>&1 || die "нет контейнера старого бота $OLD_BOT"
docker inspect "$OLD_DB"  >/dev/null 2>&1 || die "нет контейнера старой базы $OLD_DB"
docker inspect "$PANEL"   >/dev/null 2>&1 || die "нет контейнера панели $PANEL"
[[ "$(docker inspect -f '{{.State.Running}}' "$PANEL")" == "true" ]] || die "контейнер панели $PANEL не запущен"

OLD_BOT_IMAGE="$(docker inspect -f '{{.Config.Image}}' "$OLD_BOT")"
OLD_BOT_PROJECT="$(project_of "$OLD_BOT")"
OLD_BOT_NETWORK="$(docker inspect -f '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{println}}{{end}}' "$OLD_BOT" | head -1)"
OLD_BOT_RESTART="$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$OLD_BOT")"
OLD_DB_IMAGE="$(docker inspect -f '{{.Config.Image}}' "$OLD_DB")"
OLD_DB_PROJECT="$(project_of "$OLD_DB")"
OLD_DB_RESTART="$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$OLD_DB")"
OLD_DB_USER="$(container_env "$OLD_DB" POSTGRES_USER)"; OLD_DB_USER="${OLD_DB_USER:-user}"
OLD_DB_NAME="$(container_env "$OLD_DB" POSTGRES_DB)";   OLD_DB_NAME="${OLD_DB_NAME:-usersdb}"
OLD_DB_HOST="$(container_env "$OLD_BOT" DATABASE_URL | sed -E 's#^[a-z+]+://([^@/]*@)?([^:/?]+).*#\2#')"
OLD_DB_HOST="${OLD_DB_HOST:-postgres}"

# Compose выбирает контейнеры по меткам project+service. Если оставить имя
# проекта как у старого стека, compose будет считать старую БД своей и может
# остановить/удалить её. Поэтому автоматически уходим на имя с суффиксом _up.
if [[ "$PROJECT" == "$OLD_BOT_PROJECT" || "$PROJECT" == "$OLD_DB_PROJECT" ]]; then
  if [[ "$PROJECT_SET" == "1" ]]; then
    die "проект $PROJECT совпадает со старым стеком (${OLD_BOT_PROJECT:-?}). Задайте другое имя через --project"
  fi
  PROJECT="${PROJECT}_up"
fi

MOUNT_LINE="$(docker inspect -f '{{range .Mounts}}{{.Type}}|{{.Name}}|{{.Source}}|{{.Destination}}{{println}}{{end}}' "$OLD_DB" \
  | awk -F'|' '$4 == "/var/lib/postgresql/data" || $4 == "/var/lib/postgresql" {print; exit}')"
[[ -n "$MOUNT_LINE" ]] || die "у $OLD_DB не найден том с данными PostgreSQL"
IFS='|' read -r OLD_DB_MOUNT_TYPE OLD_DB_VOLUME OLD_DB_MOUNT_SOURCE OLD_DB_MOUNT_DEST <<< "$MOUNT_LINE"
if [[ "$OLD_DB_MOUNT_TYPE" == "volume" ]]; then
  DOCKER_ROOT="$(docker info -f '{{.DockerRootDir}}')"
  OLD_DB_MOUNT_SOURCE="$DOCKER_ROOT/volumes/$OLD_DB_VOLUME/_data"
fi
[[ -n "$OLD_DB_MOUNT_SOURCE" ]] || die "не удалось определить путь тома старой базы"

# Новый стек не должен незаметно занять чужие контейнеры с теми же именами.
for name in proxymaster_bot postgres traefik; do
  if docker inspect "$name" >/dev/null 2>&1; then
    proj="$(project_of "$name")"
    state="$(docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null || true)"
    if [[ "$proj" != "$PROJECT" ]]; then
      HARD_CONFLICTS+=("контейнер $name уже существует в проекте ${proj:-<без проекта>} (status=$state)")
    elif [[ "$name" == "proxymaster_bot" ]]; then
      img="$(docker inspect -f '{{.Config.Image}}' "$name")"
      if [[ "$img" != "$OLD_BOT_IMAGE" ]]; then
        if [[ "$state" == "running" ]]; then
          HARD_CONFLICTS+=("proxymaster_bot уже запущен в проекте $PROJECT — обновление уже выполнено")
        else
          SOFT_CONFLICTS+=("proxymaster_bot в проекте $PROJECT остановлен (след прошлого запуска); проверьте, не осталась ли незавершённая миграция")
        fi
      fi
    else
      SOFT_CONFLICTS+=("контейнер $name уже существует в проекте $PROJECT (след прошлого запуска)")
    fi
  fi
done

# Порт 5432 у нового compose зашит, web-порт задаётся через WEB_PORT.
# Старые контейнеры будут остановлены, поэтому их порты не считаем занятыми.
check_port() {
  local port="$1" label="$2" name ports p
  local -a parts
  while IFS='|' read -r name ports; do
    [[ -z "$name" ]] && continue
    [[ "$name" == "$OLD_BOT" || "$name" == "$OLD_DB" ]] && continue
    read -r -a parts <<< "${ports//,/ }"
    for p in "${parts[@]}"; do
      if [[ "$p" == *":$port->"* ]]; then
        HARD_CONFLICTS+=("порт $port ($label) уже опубликован контейнером $name (project=$(project_of "$name"))")
        return 0
      fi
    done
  done < <(docker ps --format '{{.Names}}|{{.Ports}}')
}
check_port 5432 "postgres"
check_port "${WEB_PORT_OVERRIDE:-8081}" "web"
if [[ -n "$DOMAIN" ]]; then
  check_port 444 "traefik https"
  check_port 8090 "traefik acme"
fi

if [[ "$DRY_RUN" == "1" ]]; then
  cat <<EOF

--- План обновления (dry-run) ---
Каталог:            $APP_DIR
Старый бот:         $OLD_BOT / $OLD_BOT_IMAGE (project=${OLD_BOT_PROJECT:-?}, restart=$OLD_BOT_RESTART)
Сеть старого бота:  ${OLD_BOT_NETWORK:-<нет>}
Старая БД:          $OLD_DB / $OLD_DB_IMAGE (user=$OLD_DB_USER, db=$OLD_DB_NAME)
Том старой БД:      $OLD_DB_VOLUME ($OLD_DB_MOUNT_SOURCE)
Хост БД для отк:    $OLD_DB_HOST
Панель:             $PANEL
Новый проект:       $PROJECT
Новый том:          ${PROJECT}_pgdata
Новая сеть:         ${PROJECT}_net
Бэкап:              $BACKUP_ROOT/$TS-pre-v2
Домен:              ${DOMAIN:-<выключен>}
Перенос устройств:  $([[ "$DO_MIGRATE" == "1" ]] && echo да || echo нет)

Что будет сделано:
  1) бэкап репозитория, .env, образа, дампа и физического тома старой БД;
  2) локальные изменения -> ветка backup/pre-v2-$TS, затем reset на origin/$BRANCH;
  3) дополнение .env обязательными ключами новой версии;
  4) отдельный docker-compose.override.yml: проект $PROJECT, сеть ${PROJECT}_net, том ${PROJECT}_pgdata;
  5) сборка нового образа до остановки старого бота;
  6) остановка старого бота и БД (restart=no), финальный дамп и копия тома;
  7) запуск postgres:18.6-alpine на новом томе;
  8) migrate_old_version.sh: dry-run, затем запись мест (--no-cutover);
  9) запуск нового бота и проверка Telegram/БД;
 10) ROLLBACK.md с командами возврата.
EOF
  if ((${#HARD_CONFLICTS[@]})); then
    printf '\nЖёсткие конфликты (--force их не обходит):\n'
    printf '  - %s\n' "${HARD_CONFLICTS[@]}"
  fi
  if ((${#SOFT_CONFLICTS[@]})); then
    printf '\nМягкие конфликты (продолжение только с --force --yes):\n'
    printf '  - %s\n' "${SOFT_CONFLICTS[@]}"
  fi
  if ((${#HARD_CONFLICTS[@]} || ${#SOFT_CONFLICTS[@]})); then
    printf '\nDry-run завершён. Ничего не изменено.\n'
    exit 2
  fi
  printf '\nDry-run завершён. Ничего не изменено.\n'
  exit 0
fi

if ((${#HARD_CONFLICTS[@]})); then
  printf 'Жёсткие конфликты (--force их не обходит):\n' >&2
  printf '  - %s\n' "${HARD_CONFLICTS[@]}" >&2
  die "остановлено. Устраните конфликты имён/портов или обновите этот стек вручную"
fi
if ((${#SOFT_CONFLICTS[@]})); then
  printf 'Мягкие конфликты:\n' >&2
  printf '  - %s\n' "${SOFT_CONFLICTS[@]}" >&2
  [[ "$FORCE" == "1" && "$ASSUME_YES" == "1" ]] || die "остановлено. Для продолжения нужны --force --yes"
  warn "продолжаю с --force; проверьте, что миграция не задвоит места"
fi

mkdir -p "$BACKUP_ROOT"
exec 9>"$BACKUP_ROOT/.update_to_v2.lock"
flock -n 9 || die "уже выполняется другой запуск update_to_v2.sh"

env_get() {
  local key="$1" line value
  line="$(grep -E "^[[:space:]]*${key}[[:space:]]*=" "$ENV_FILE" | tail -1 || true)"
  [[ -n "$line" ]] || return 0
  value="${line#*=}"
  value="${value#"${value%%[![:space:]]*}"}"
  case "$value" in
    \"*\") value="${value#\"}"; value="${value%%\"*}" ;;
    \'*\') value="${value#\'}"; value="${value%%\'*}" ;;
  esac
  printf '%s' "$value" | sed -E 's/[[:space:]]+#.*$//'
}

env_set() {
  local key="$1" value="$2" tmp
  tmp="$(mktemp)"
  awk -v key="$key" -v value="$value" '
    BEGIN { done=0 }
    {
      if ($0 ~ "^[[:space:]]*" key "[[:space:]]*=") {
        if (!done) { print key "=" value; done=1 }
        next
      }
      print
    }
    END { if (!done) print key "=" value }
  ' "$ENV_FILE" > "$tmp"
  cat "$tmp" > "$ENV_FILE"
  rm -f "$tmp"
}

gen_secret() {
  if have openssl; then
    openssl rand -hex 32
  else
    od -An -tx1 -N32 /dev/urandom | tr -d ' \n'
  fi
}

backup_all() {
  BK="$BACKUP_ROOT/$TS-pre-v2"
  mkdir -p "$BK"
  chmod 700 "$BK"
  log "бэкапы: $BK"

  local avail_kb need_kb=2097152
  avail_kb="$(df -Pk "$BK" | awk 'NR==2 {print $4}')"
  if [[ -n "$OLD_DB_MOUNT_SOURCE" && -d "$OLD_DB_MOUNT_SOURCE" ]]; then
    local vol_kb
    vol_kb="$(du -sk "$OLD_DB_MOUNT_SOURCE" 2>/dev/null | awk '{print $1}' || true)"
    [[ "$vol_kb" =~ ^[0-9]+$ ]] && need_kb=$((vol_kb * 2 + 2097152))
  fi
  if [[ "$avail_kb" =~ ^[0-9]+$ ]] && ((avail_kb < need_kb)); then
    die "мало свободного места для бэкапов: доступно $((avail_kb / 1024)) МБ, нужно примерно $((need_kb / 1024)) МБ"
  fi

  tar czf "$BK/repo.tar.gz" -C "$(dirname "$APP_DIR")" "$(basename "$APP_DIR")"
  cp "$ENV_FILE" "$BK/env.old"; chmod 600 "$BK/env.old"
  cp "$APP_DIR/docker-compose.yml" "$BK/docker-compose.old.yml"
  [[ -f "$APP_DIR/docker-compose.override.yml" ]] \
    && cp "$APP_DIR/docker-compose.override.yml" "$BK/docker-compose.override.old.yml"

  docker inspect "$OLD_BOT" > "$BK/old-bot.inspect.json"
  docker inspect "$OLD_DB"  > "$BK/old-db.inspect.json"
  docker image inspect "$OLD_BOT_IMAGE" > "$BK/old-image.inspect.json"
  docker version > "$BK/docker-version.txt" 2>&1 || true
  docker compose version > "$BK/compose-version.txt" 2>&1 || true
  git -C "$APP_DIR" rev-parse HEAD > "$BK/old-commit.txt"
  git -C "$APP_DIR" branch --show-current > "$BK/old-branch.txt" 2>/dev/null || true

  if docker tag "$OLD_BOT_IMAGE" "proxy-app:pre-v2-$TS" >/dev/null 2>&1; then
    if ! docker save "proxy-app:pre-v2-$TS" | gzip > "$BK/old-image.tar.gz"; then
      rm -f "$BK/old-image.tar.gz"
      die "не удалось сохранить образ старого бота"
    fi
  else
    warn "не удалось пометить старый образ; сохраняю по исходному имени"
    if ! docker save "$OLD_BOT_IMAGE" | gzip > "$BK/old-image.tar.gz"; then
      rm -f "$BK/old-image.tar.gz"
      die "не удалось сохранить образ старого бота по исходному имени"
    fi
  fi

  for f in promo_codes.txt ref.txt; do
    if [[ -f "$APP_DIR/$f" ]]; then
      cp "$APP_DIR/$f" "$BK/$f"
      chmod 600 "$BK/$f"
    fi
  done

  dump_db "$BK/usersdb.dump" || die "первичный дамп старой БД не удался"
  log "первичный дамп старой БД готов"
}

# Временная база поднимается не на оригинальном томе, а на распакованной
# копии physical-backup'а: второй postgres не должен даже случайно изменить
# боевой том старой БД.
start_temp_old_db() {
  docker rm -f "${OLD_DB}_readonly" >/dev/null 2>&1 || true
  TEMP_DB_VOLUME="${PROJECT}_oldcopy"

  if [[ ! -f "$BK/old-volume.tar.gz" ]]; then
    # Снимок ещё не делали (старая БД уже была остановлена к началу скрипта).
    running "$OLD_DB" && return 1
    physical_backup || return 1
  fi

  # Копию каждый раз распаковываем заново, чтобы не тащить следы прошлого
  # запуска; оригинальный том при этом не трогается.
  docker volume rm -f "$TEMP_DB_VOLUME" >/dev/null 2>&1 || true
  docker volume create "$TEMP_DB_VOLUME" >/dev/null || return 1
  if ! docker run --rm --entrypoint sh \
      -v "$TEMP_DB_VOLUME:/target" -v "$BK:/backup:ro" \
      "$OLD_DB_IMAGE" -c 'tar xzf /backup/old-volume.tar.gz -C /target --strip-components=1' >/dev/null; then
    docker volume rm -f "$TEMP_DB_VOLUME" >/dev/null 2>&1 || true
    return 1
  fi

  docker run -d --name "${OLD_DB}_readonly" --network none \
    -e POSTGRES_USER=unused -e POSTGRES_PASSWORD=unused \
    -v "$TEMP_DB_VOLUME:$OLD_DB_MOUNT_DEST" \
    "$OLD_DB_IMAGE" >/dev/null || return 1

  local i
  for i in $(seq 1 60); do
    docker exec "${OLD_DB}_readonly" pg_isready -U "$OLD_DB_USER" -d "$OLD_DB_NAME" >/dev/null 2>&1 && return 0
    sleep 1
  done
  return 1
}

stop_temp_old_db() { docker rm -f "${OLD_DB}_readonly" >/dev/null 2>&1 || true; }

cleanup_temp_old_db() {
  stop_temp_old_db
  if [[ -n "${TEMP_DB_VOLUME:-}" && "${TEMP_DB_VOLUME}" == "${PROJECT}_oldcopy" ]]; then
    docker volume rm -f "$TEMP_DB_VOLUME" >/dev/null 2>&1 || true
  fi
}

dump_db() {
  local out="$1" tmp="$1.tmp"
  rm -f "$tmp"
  if running "$OLD_DB"; then
    docker exec "$OLD_DB" pg_dump -U "$OLD_DB_USER" -d "$OLD_DB_NAME" -Fc > "$tmp" \
      || { rm -f "$tmp"; return 1; }
  else
    start_temp_old_db || { stop_temp_old_db; return 1; }
    docker exec "${OLD_DB}_readonly" pg_dump -U "$OLD_DB_USER" -d "$OLD_DB_NAME" -Fc > "$tmp" \
      || { stop_temp_old_db; rm -f "$tmp"; return 1; }
    stop_temp_old_db
  fi
  mv "$tmp" "$out"
}

physical_backup() {
  local c
  if [[ -n "$OLD_DB_VOLUME" ]]; then
    while IFS= read -r c; do
      [[ -z "$c" || "$c" == "$OLD_DB" ]] && continue
      if running "$c"; then
        warn "том $OLD_DB_VOLUME всё ещё используется контейнером $c"
        return 1
      fi
    done < <(docker ps -a --filter "volume=$OLD_DB_VOLUME" --format '{{.Names}}')
  fi
  tar czf "$BK/old-volume.tar.gz" \
    -C "$(dirname "$OLD_DB_MOUNT_SOURCE")" "$(basename "$OLD_DB_MOUNT_SOURCE")" || return 1
  log "физическая копия тома: $BK/old-volume.tar.gz"
}

git_backup_update() {
  cd "$APP_DIR"
  local cur backup
  cur="$(git symbolic-ref --quiet --short HEAD || true)"
  backup="backup/pre-v2-$TS"
  git branch "$backup" HEAD >/dev/null || die "не удалось создать ветку $backup"

  if [[ -n "$(git status --porcelain)" ]]; then
    log "локальные изменения сохраняю в ветку $backup"
    git checkout "$backup" >/dev/null
    git add -A
    # Секреты и данные не коммитим: они уже лежат в бэкапе отдельными файлами.
    for p in .env docker-compose.override.yml promo_codes.txt ref.txt; do
      git reset -- "$p" >/dev/null 2>&1 || true
    done
    if git diff --cached --quiet; then
      log "значимых локальных изменений нет, ветка $backup пустая"
    else
      git -c user.name="ProxyMaster update" -c user.email="root@localhost" \
        commit -m "backup: local changes before v2 update $TS" >/dev/null
    fi
    if [[ -n "$cur" ]]; then git checkout "$cur" >/dev/null; else git checkout "$BRANCH" >/dev/null; fi
  fi

  git fetch --prune origin "$BRANCH" || die "не удалось получить origin/$BRANCH"
  git show-ref --verify --quiet "refs/remotes/origin/$BRANCH" \
    || die "в origin нет ветки $BRANCH (проверьте --branch)"
  git checkout "$BRANCH" >/dev/null
  git reset --hard "origin/$BRANCH" >/dev/null
  log "код обновлён: $(cat "$BK/old-commit.txt") -> $(git rev-parse --short HEAD)"
}

env_migrate() {
  local provider d1 d2 d3 d4 d
  provider="$(env_get PAYMENT_PROVIDER | tr '[:upper:]' '[:lower:]')"
  [[ -n "$provider" ]] || die "в .env не задан PAYMENT_PROVIDER"
  env_set PAYMENT_PROVIDER "$provider"

  case "$provider" in
    yookassa|youkassa)
      # В старом коде ключи назывались YOUKASSA_* (одна O). Новый код
      # использует те же имена, но дополнительно читает YOOKASSA_AUTOPAY.
      local suffix src dst
      for suffix in SHOP_ID SECRET_KEY RETURN_URL; do
        dst="YOUKASSA_$suffix"
        if [[ -z "$(env_get "$dst")" ]]; then
          src="YOOKASSA_$suffix"
          [[ -n "$(env_get "$src")" ]] && env_set "$dst" "$(env_get "$src")"
        fi
      done
      [[ -n "$(env_get YOUKASSA_SHOP_ID)" && -n "$(env_get YOUKASSA_SECRET_KEY)" ]] \
        || die "для ЮKassa нужны YOUKASSA_SHOP_ID и YOUKASSA_SECRET_KEY"
      [[ -n "$(env_get YOUKASSA_RETURN_URL)" ]] || env_set YOUKASSA_RETURN_URL "https://t.me/"
      env_set YOOKASSA_AUTOPAY true
      ;;
    platega)
      for d in PLATEGA_MERCHANT_ID PLATEGA_API_KEY PLATEGA_RETURN_URL; do
        [[ -n "$(env_get "$d")" ]] || die "для Platega не задан $d"
      done
      ;;
    fake) ;;
    *) die "PAYMENT_PROVIDER=$provider не поддерживается: нужен platega, fake или yookassa через мост" ;;
  esac

  if [[ -z "$(env_get POSTGRES_PASSWORD)" ]]; then
    env_set POSTGRES_PASSWORD "$(gen_secret)"
    log "сгенерирован новый POSTGRES_PASSWORD"
  fi
  env_set COMPOSE_PROJECT_NAME "$PROJECT"
  env_set WEB_ADDR ":8080"

  local host_port="${WEB_PORT_OVERRIDE:-8081}"
  [[ "$host_port" =~ ^[0-9]+$ ]] || die "--web-port должен быть числом"
  if [[ "$PUBLIC_WEB" == "1" ]]; then
    env_set WEB_PORT "$host_port"
  else
    env_set WEB_PORT "127.0.0.1:$host_port"
  fi

  if [[ -n "$DOMAIN" ]]; then
    [[ -n "$ACME_EMAIL" ]] || ACME_EMAIL="$(env_get ACME_EMAIL)"
    [[ -n "$ACME_EMAIL" ]] || die "с --domain нужен --acme-email"
    env_set AUTO_CONNECT_SUB "$DOMAIN"
    env_set AUTO_CONNECT_DOMAIN "$DOMAIN"
    env_set ACME_EMAIL "$ACME_EMAIL"
    [[ -n "$(env_get LINK_SECRET)" ]] || env_set LINK_SECRET "$(gen_secret)"
  else
    # Домен не задан: Traefik остаётся в профиле domain и не поднимается.
    # Существующие значения не затираем — вдруг там уже рабочий домен.
    [[ -n "$(env_get AUTO_CONNECT_DOMAIN)" ]] || env_set AUTO_CONNECT_DOMAIN "localhost"
    if [[ -n "$(env_get AUTO_CONNECT_SUB)" ]]; then
      warn "AUTO_CONNECT_SUB уже задан, но Traefik выключен; автоподключение заработает только с --domain"
    fi
  fi

  if [[ -n "$PRICE_OVERRIDE" ]]; then
    [[ "$PRICE_OVERRIDE" =~ ^[0-9]+$ ]] && ((10#$PRICE_OVERRIDE > 0)) || die "--price должен быть положительным числом"
    env_set PRICE_PER_MONTH "$PRICE_OVERRIDE"
  fi
  [[ -n "$(env_get PRICE_PER_MONTH)" ]] || env_set PRICE_PER_MONTH 200

  if [[ -n "$DAYS_OVERRIDE" ]]; then
    IFS=',' read -r d1 d2 d3 d4 <<< "$DAYS_OVERRIDE"
    [[ -n "$d1" && -n "$d2" && -n "$d3" && -n "$d4" ]] || die "--days требует четыре значения: 7,30,60,90"
    for d in "$d1" "$d2" "$d3" "$d4"; do
      [[ "$d" =~ ^[0-9]+$ ]] && ((10#$d > 0)) || die "неверное значение в --days: $d"
    done
    env_set INITIAL_PRICE_DAYS1 "$d1"
    env_set INITIAL_PRICE_DAYS2 "$d2"
    env_set INITIAL_PRICE_DAYS3 "$d3"
    env_set INITIAL_PRICE_DAYS4 "$d4"
  fi
  [[ -n "$(env_get INITIAL_PRICE_DAYS1)" ]] || env_set INITIAL_PRICE_DAYS1 30
  [[ -n "$(env_get INITIAL_PRICE_DAYS2)" ]] || env_set INITIAL_PRICE_DAYS2 60
  [[ -n "$(env_get INITIAL_PRICE_DAYS3)" ]] || env_set INITIAL_PRICE_DAYS3 90
  [[ -n "$(env_get INITIAL_PRICE_DAYS4)" ]] || env_set INITIAL_PRICE_DAYS4 120
  [[ -n "$(env_get DEVICE_LIMIT)" ]] || env_set DEVICE_LIMIT 2
  [[ -n "$(env_get MAX_DEVICE_LIMIT)" ]] || env_set MAX_DEVICE_LIMIT 5
  [[ -n "$(env_get EXTRA_DEVICE_PRICE)" ]] || env_set EXTRA_DEVICE_PRICE 50
  [[ -n "$(env_get TRIAL_DAYS)" ]] || env_set TRIAL_DAYS 30
  [[ -n "$(env_get PROVIDER_NAME)" ]] || env_set PROVIDER_NAME "ProxyMaster"

  local k
  for k in REMNA_PANEL REMNA_TOKEN REMNA_SQUAD_UUID TELEGRAM_TOKEN TELEGRAM_SUPPORT; do
    [[ -n "$(env_get "$k")" ]] || die "в .env нет обязательного $k"
  done
  [[ "$(env_get PRICE_PER_MONTH)" =~ ^[0-9]+$ ]] || die "PRICE_PER_MONTH должен быть числом"
  [[ "$(env_get DEVICE_LIMIT)" =~ ^[0-9]+$ ]] || die "DEVICE_LIMIT должен быть числом"
  ((10#$(env_get PRICE_PER_MONTH) > 0)) || die "PRICE_PER_MONTH должен быть больше нуля"
  ((10#$(env_get DEVICE_LIMIT) > 0)) || die "DEVICE_LIMIT должен быть больше нуля"
  chmod 600 "$ENV_FILE"
  log ".env дополнен ключами новой версии"
}

write_override() {
  local f="$APP_DIR/docker-compose.override.yml"
  {
    echo 'services:'
    echo '  bot:'
    echo '    depends_on:'
    echo '      postgres:'
    echo '        condition: service_healthy'
    if [[ -z "$DOMAIN" ]]; then
      echo '  # Пока домен автоподключения не привязан, Traefik не поднимаем.'
      echo '  traefik:'
      echo '    profiles: ["domain"]'
    fi
    echo 'volumes:'
    echo '  postgres_data:'
    echo "    name: ${PROJECT}_pgdata"
    echo '    external: true'
    echo '  traefik_acme:'
    echo "    name: ${PROJECT}_traefik_acme"
    echo 'networks:'
    echo '  default:'
    echo "    name: ${PROJECT}_net"
  } > "$f"

  mkdir -p "$APP_DIR/.git/info"
  grep -qxF 'docker-compose.override.yml' "$APP_DIR/.git/info/exclude" 2>/dev/null \
    || echo 'docker-compose.override.yml' >> "$APP_DIR/.git/info/exclude"
  log "compose-проект $PROJECT, сеть ${PROJECT}_net, том ${PROJECT}_pgdata"
}

prepare_volumes() {
  if ! docker volume inspect "${PROJECT}_pgdata" >/dev/null 2>&1; then
    NEW_VOLUME_CREATED=1
  fi
  docker volume create "${PROJECT}_pgdata" >/dev/null \
    || die "не удалось создать том ${PROJECT}_pgdata"
  if [[ -n "$DOMAIN" ]]; then
    docker volume create "${PROJECT}_traefik_acme" >/dev/null \
      || die "не удалось создать том ${PROJECT}_traefik_acme"
  fi
  log "тома готовы: ${PROJECT}_pgdata$([[ -n "$DOMAIN" ]] && echo ", ${PROJECT}_traefik_acme")"
}

build_new_image() {
  cd "$APP_DIR"
  log "собираю новый образ (старый бот ещё работает)"
  if ! docker compose -p "$PROJECT" build bot 2>&1 | tee "$BK/build.log"; then
    die "сборка не прошла; старый стек не тронут"
  fi
}

stop_old_stack() {
  log "останавливаю старый стек"
  docker update --restart=no "$OLD_BOT" "$OLD_DB" >/dev/null 2>&1 || true
  OLD_STOPPED=1

  if running "$OLD_BOT"; then
    docker stop "$OLD_BOT" >/dev/null || fail_after_stop "не удалось остановить старого бота"
  fi
  dump_db "$BK/usersdb-final.dump" || fail_after_stop "финальный дамп старой БД не удался"
  if running "$OLD_DB"; then
    docker stop "$OLD_DB" >/dev/null || fail_after_stop "не удалось остановить старую БД"
  fi
  stop_temp_old_db
  physical_backup || fail_after_stop "физическая копия тома старой БД не удалась"
  log "старый стек остановлен, бэкапы готовы"
}

start_new_db() {
  docker volume create "${PROJECT}_pgdata" >/dev/null \
    || fail_after_stop "не удалось создать том ${PROJECT}_pgdata"
  log "запускаю postgres нового проекта"
  if ! docker compose -p "$PROJECT" up -d postgres 2>&1 | tee "$BK/postgres-up.log"; then
    fail_after_stop "не удалось поднять postgres нового проекта"
  fi

  local i st
  for i in $(seq 1 90); do
    st="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' postgres 2>/dev/null || true)"
    [[ "$st" == "healthy" ]] && return 0
    [[ "$st" == "exited" || "$st" == "dead" ]] && break
    sleep 2
  done
  docker logs --tail 50 postgres 2>&1 || true
  fail_after_stop "postgres не стал healthy"
}

migrate_devices() {
  if [[ "$DO_MIGRATE" != "1" ]]; then
    warn "перенос устройств отключён (--no-migrate)"
    return 0
  fi
  cd "$APP_DIR"
  local args rc answer old_ref
  old_ref="$OLD_DB"
  if ! running "$OLD_DB"; then
    log "поднимаю временную копию старой БД для чтения"
    start_temp_old_db || fail_after_stop "не удалось поднять временную копию старой БД"
    old_ref="${OLD_DB}_readonly"
  fi
  args=(--old="$old_ref" --panel="$PANEL" --old-bot="$OLD_BOT" --app=postgres --base-limit="$(env_get DEVICE_LIMIT)")

  log "пробный прогон миграции"
  set +e
  bash ./other/database/migrate_old_version.sh --dry-run -y "${args[@]}" 2>&1 | tee "$BK/migrate-dry-run.log"
  rc=${PIPESTATUS[0]}
  set -e
  ((rc == 0)) || fail_after_stop "пробная миграция не прошла"

  if [[ "$ASSUME_YES" != "1" ]]; then
    read -r -p "Применить миграцию? (y/N): " answer || true
    [[ "${answer:-n}" == "y" || "${answer:-n}" == "Y" ]] || fail_after_stop "миграция отменена"
  fi

  log "применяю миграцию"
  set +e
  bash ./other/database/migrate_old_version.sh --no-cutover -y "${args[@]}" 2>&1 | tee "$BK/migrate-apply.log"
  rc=${PIPESTATUS[0]}
  set -e
  ((rc == 0)) || fail_after_stop "миграция не прошла"

  cleanup_temp_old_db
  log "миграция устройств завершена"
}

start_new_bot() {
  log "запускаю нового бота"
  if ! docker compose -p "$PROJECT" up -d bot 2>&1 | tee "$BK/bot-up.log"; then
    fail_after_stop "docker compose up bot не удался"
  fi

  local i st
  for i in $(seq 1 60); do
    st="$(docker inspect -f '{{.State.Status}}' proxymaster_bot 2>/dev/null || true)"
    [[ "$st" == "running" ]] && break
    sleep 1
  done
  if [[ "$st" != "running" ]]; then
    docker logs --tail 60 proxymaster_bot 2>&1 || true
    fail_after_stop "новый бот не запустился"
  fi

  sleep 5
  if docker logs --tail 60 proxymaster_bot 2>&1 | grep -qE 'panic|FATAL'; then
    docker logs --tail 60 proxymaster_bot 2>&1 || true
    fail_after_stop "новый бот пишет panic/FATAL"
  fi
  NEW_BOT_STARTED=1
}

verify_stack() {
  local count tg
  log "проверка нового стека"
  docker compose -p "$PROJECT" ps
  docker inspect proxymaster_bot --format 'бот: status={{.State.Status}} restarts={{.RestartCount}}'
  count="$(device_count)"
  echo "устройств в новой БД: $count"

  tg="$(docker exec proxymaster_bot sh -c 'if command -v wget >/dev/null 2>&1; then wget -qO- "https://api.telegram.org/bot$TELEGRAM_TOKEN/getMe"; elif command -v curl >/dev/null 2>&1; then curl -fsS "https://api.telegram.org/bot$TELEGRAM_TOKEN/getMe"; fi' 2>/dev/null || true)"
  if printf '%s' "$tg" | grep -q '"ok":true'; then
    echo "Telegram: getMe OK"
  else
    warn "Telegram getMe не ответил; проверьте docker logs proxymaster_bot"
  fi
  if docker logs --tail 100 proxymaster_bot 2>&1 | grep -q 'автоплатёж выключен'; then
    warn "автоплатёж ЮKassa не включился: проверьте YOUKASSA_* и YOOKASSA_AUTOPAY"
  fi

  docker ps -a --filter "name=$OLD_BOT" --filter "name=$OLD_DB" \
    --format 'старый контейнер: {{.Names}} — {{.Status}}'
  docker network inspect "${PROJECT}_net" \
    --format 'сеть {{.Name}}: {{range .Containers}}{{.Name}} {{end}}' 2>/dev/null || true
}

rollback_old() {
  warn "возвращаю старый стек"
  # Сначала убираем временную копию старой БД: два postgres на одном томе
  # недопустимы.
  stop_temp_old_db

  docker compose -p "$PROJECT" down --remove-orphans >/dev/null 2>&1 || true
  local c
  for c in proxymaster_bot postgres; do
    if docker inspect "$c" >/dev/null 2>&1 && [[ "$(project_of "$c")" == "$PROJECT" ]]; then
      docker rm -f "$c" >/dev/null 2>&1 || true
    fi
  done
  if [[ "$NEW_VOLUME_CREATED" == "1" ]]; then
    docker volume rm "${PROJECT}_pgdata" >/dev/null 2>&1 || true
  fi

  if ! running "$OLD_DB"; then
    docker start "$OLD_DB" >/dev/null 2>&1 || true
  fi
  local i
  for i in $(seq 1 60); do
    docker exec "$OLD_DB" pg_isready -U "$OLD_DB_USER" -d "$OLD_DB_NAME" >/dev/null 2>&1 && break
    sleep 1
  done

  if [[ -n "$OLD_BOT_NETWORK" ]]; then
    docker network connect --alias "$OLD_DB_HOST" "$OLD_BOT_NETWORK" "$OLD_DB" >/dev/null 2>&1 || true
  fi
  if ! running "$OLD_BOT"; then
    docker start "$OLD_BOT" >/dev/null 2>&1 || true
  fi
  if [[ -n "$OLD_BOT_RESTART" && "$OLD_BOT_RESTART" != "no" ]]; then
    docker update --restart="$OLD_BOT_RESTART" "$OLD_BOT" >/dev/null 2>&1 || true
  fi
  if [[ -n "$OLD_DB_RESTART" && "$OLD_DB_RESTART" != "no" ]]; then
    docker update --restart="$OLD_DB_RESTART" "$OLD_DB" >/dev/null 2>&1 || true
  fi
  warn "старый стек возвращён; проверьте: docker logs --tail 50 $OLD_BOT"
}

fail_after_stop() {
  # Откат делает cleanup_on_exit по EXIT-ловушке: так он сработает и при
  # неожиданной ошибке set -e, а не только при явном вызове.
  die "$1"
}

cleanup_on_exit() {
  local rc=$?
  cleanup_temp_old_db >/dev/null 2>&1 || true
  if ((rc != 0)) && [[ "$OLD_STOPPED" == "1" && "$NEW_BOT_STARTED" != "1" && "$ROLLED_BACK" != "1" ]]; then
    ROLLED_BACK=1
    rollback_old || true
  fi
  exit "$rc"
}

write_rollback() {
  local count="$1"
  cat > "$BK/ROLLBACK.md" <<EOF
# ProxyMaster: обновление $TS

## Было / стало
- Каталог: $APP_DIR
- Старый бот: $OLD_BOT ($OLD_BOT_IMAGE)
- Старый коммит: $(cat "$BK/old-commit.txt")
- Новый коммит: $(git -C "$APP_DIR" rev-parse HEAD)
- Локальные изменения: ветка backup/pre-v2-$TS (если были)

## Бэкапы (в этой папке)
- repo.tar.gz              — весь старый каталог
- old-image.tar.gz         — docker-образ старого бота
- usersdb.dump             — дамп до остановки бота
- usersdb-final.dump       — финальный дамп после остановки бота
- old-volume.tar.gz        — физическая копия тома старой БД
- env.old                  — старый .env
- docker-compose.old.yml   — старый compose
- old-bot.inspect.json     — inspect старого бота
- old-db.inspect.json      — inspect старой БД
- migrate-dry-run.log      — пробная миграция
- migrate-apply.log        — применённая миграция
- build.log                — сборка нового образа

## Что перенесено
- Устройств в новой БД: $count
- Источник: оплаченные доп. места старой БД + лимиты устройств из панели.
- Старые контейнеры остановлены с restart=no, их тома не удалены.

## Текущий стек
- Compose project: $PROJECT
- Сеть: ${PROJECT}_net
- postgres: контейнер postgres, том ${PROJECT}_pgdata (external)
- bot: контейнер proxymaster_bot, restart unless-stopped
- traefik: $([[ -n "$DOMAIN" ]] && echo "включён, домен $DOMAIN" || echo "выключен профилем domain")
- старый бот $OLD_BOT и БД $OLD_DB остановлены

## Откат
1) Остановить новый стек:
   cd $APP_DIR
   docker compose -p $PROJECT stop bot postgres
2) Поднять старую БД и старого бота:
   docker start $OLD_DB
EOF
  if [[ -n "$OLD_BOT_NETWORK" ]]; then
    cat >> "$BK/ROLLBACK.md" <<EOF
   # если старой БД нет в сети старого бота:
   docker network connect --alias $OLD_DB_HOST $OLD_BOT_NETWORK $OLD_DB
EOF
  fi
  cat >> "$BK/ROLLBACK.md" <<EOF
   docker start $OLD_BOT
3) Вернуть политики рестарта:
   docker update --restart=${OLD_BOT_RESTART:-no} $OLD_BOT
   docker update --restart=${OLD_DB_RESTART:-no} $OLD_DB
4) Проверить: docker ps; docker logs --tail 50 $OLD_BOT

## Если старый том всё-таки повреждён (крайний случай)
Остановить старую БД и распаковать физическую копию:
  docker stop $OLD_DB
  rm -rf $OLD_DB_MOUNT_SOURCE
  mkdir -p $OLD_DB_MOUNT_SOURCE
  tar xzf $BK/old-volume.tar.gz -C $OLD_DB_MOUNT_SOURCE --strip-components=1
  docker start $OLD_DB

## Включить автоподключение, когда появится домен
Перезапустить скрипт с доменом:
  ./update_to_v2.sh --app-dir $APP_DIR --domain <домен> --acme-email <email> --yes
Или вручную:
  1) в .env задать AUTO_CONNECT_SUB, AUTO_CONNECT_DOMAIN, LINK_SECRET, ACME_EMAIL;
  2) docker compose -p $PROJECT --profile domain up -d traefik;
  3) настроить nginx: ACME на 127.0.0.1:8090, домен на 127.0.0.1:444;
  4) docker compose -p $PROJECT up -d --force-recreate bot.

## Важно
- Промокоды/рефералка, если были в старом коде, в новый upstream не входят.
  Код сохранён в ветке backup/pre-v2-$TS, данные — в promo_codes.txt/ref.txt.
- Если PAYMENT_PROVIDER=yookassa, используется временный мост YOOKASSA_AUTOPAY=true
  и ключи YOUKASSA_* (одна O). После обновления проверьте реальную оплату.
- Секреты из .env не печатаются в логах скрипта.
EOF
  chmod 600 "$BK/ROLLBACK.md"
}

main() {
  backup_all
  env_migrate
  git_backup_update
  write_override
  prepare_volumes
  build_new_image
  stop_old_stack
  start_new_db
  migrate_devices
  start_new_bot

  local count
  count="$(device_count)"
  write_rollback "$count"
  verify_stack

  log "готово"
  echo "Бэкап:              $BK"
  echo "Новый проект:       $PROJECT"
  echo "Устройств в БД:     $count"
  echo "Инструкция отката:  $BK/ROLLBACK.md"
  echo "Проверьте /start в Telegram и реальную оплату."
}

trap cleanup_on_exit EXIT
main "$@"
