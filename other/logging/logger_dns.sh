#!/bin/bash
set -e

COMPOSE_FILE="/root/1/docker-compose.yml"
LOG_DIR="/opt/remnanode/logs"   # НЕ /var/log - на некоторых серверах это tmpfs и стирается при ребуте
CONTAINER_MOUNT="/var/log/remnanode"

echo "=== 1. Проверка файла compose ==="
if [ ! -f "$COMPOSE_FILE" ]; then
    echo "Ошибка: $COMPOSE_FILE не найден."
    exit 1
fi

echo "=== 2. Бэкап docker-compose.yml ==="
cp "$COMPOSE_FILE" "${COMPOSE_FILE}.bak.$(date +%s)"
echo "Бэкап сохранён рядом с оригиналом."

echo "=== 3. Создание папок под логи ==="
mkdir -p "$LOG_DIR/archive"
touch "$LOG_DIR/access.log" "$LOG_DIR/error.log"
chmod 644 "$LOG_DIR/access.log" "$LOG_DIR/error.log"

echo "=== 4. Добавление volumes в docker-compose.yml (если ещё нет) ==="
if grep -q "$CONTAINER_MOUNT" "$COMPOSE_FILE"; then
    echo "Volume для логов уже есть в compose-файле - пропускаю."
else
    if grep -q "^\s*volumes:" "$COMPOSE_FILE"; then
        # Блок volumes уже существует - добавляем строку внутрь него
        sed -i "/^\s*volumes:/a\\      - '${LOG_DIR}:${CONTAINER_MOUNT}'" "$COMPOSE_FILE"
    else
        # Блока volumes нет - создаём его после строки restart: always
        sed -i "/restart: always/a\\    volumes:\\n      - '${LOG_DIR}:${CONTAINER_MOUNT}'" "$COMPOSE_FILE"
    fi
    echo "Volume добавлен: ${LOG_DIR} -> ${CONTAINER_MOUNT}"
fi

echo "=== 5. Установка logrotate (если не стоит) ==="
if ! command -v logrotate &> /dev/null; then
    apt update -qq && apt install -y logrotate
fi

echo "=== 6. Конфиг logrotate ==="
cat > /etc/logrotate.d/remnanode << EOF
${LOG_DIR}/*.log {
    daily
    rotate 1
    missingok
    notifempty
    copytruncate
    sharedscripts
    postrotate
        DATEPART=\$(date +%F)
        DEST="${LOG_DIR}/archive/\$DATEPART"
        mkdir -p "\$DEST"
        for f in ${LOG_DIR}/*.log; do
            [ -e "\$f" ] || continue
            [ -s "\$f" ] || continue
            fname=\$(basename "\$f")
            cp "\$f" "\$DEST/\$fname"
            gzip -f "\$DEST/\$fname"
        done
        rm -f ${LOG_DIR}/*.log.1
        find ${LOG_DIR}/archive -mindepth 1 -maxdepth 1 -type d ! -name "\$DATEPART" -exec rm -rf {} +
    endscript
}
EOF
echo "Конфиг logrotate записан в /etc/logrotate.d/remnanode"

echo "=== 7. Применение docker-compose ==="
cd /root/1
docker compose up -d

echo ""
echo "=== ГОТОВО ==="
echo "Volume смонтирован: ${LOG_DIR} -> ${CONTAINER_MOUNT}"
echo ""
echo "⚠️  ВАЖНО: не забудь прописать пути в JSON-конфиге Xray (в панели Remnawave), блок \"log\":"
echo '    "access": "/var/log/remnanode/access.log",'
echo '    "error": "/var/log/remnanode/error.log",'
echo '    "loglevel": "warning"'
echo ""
echo "Проверить ротацию вручную:"
echo "  sudo logrotate -f /etc/logrotate.d/remnanode"
echo "  ls -la ${LOG_DIR}/archive/"
