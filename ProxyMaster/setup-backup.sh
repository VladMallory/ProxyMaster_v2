#!/bin/bash

# Скрипт для полной настройки автоматических бекапов PostgreSQL из Docker
# Создаёт все необходимые файлы, папки и настраивает cron задачу

set -e

echo "================================"
echo "Настройка автоматических бекапов"
echo "================================"
echo ""

# Определяем путь к проекту (где находится этот скрипт)
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="$PROJECT_DIR/backups"
LOGS_DIR="$PROJECT_DIR/logs"
BACKUP_SCRIPT="$PROJECT_DIR/backup.sh"

echo "📁 Проект расположен: $PROJECT_DIR"
echo ""

# Шаг 1: Создаём необходимые папки
echo "📂 Создание папок..."
mkdir -p "$BACKUP_DIR"
mkdir -p "$LOGS_DIR"
echo "✓ Папки созданы: $BACKUP_DIR и $LOGS_DIR"
echo ""

# Шаг 2: Создаём скрипт бекапа
echo "📝 Создание скрипта бекапа..."
cat > "$BACKUP_SCRIPT" << 'EOF'
#!/bin/bash

# Скрипт для автоматического бекапа базы данных PostgreSQL из Docker контейнера
# Использует pg_dump для создания дампа БД и сохраняет его в папку backups/ с датой

set -e

# Переменные конфигурации
BACKUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/backups"
LOGS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/logs"
CONTAINER_NAME="users_postgres"
DB_USER="user"
DB_NAME="usersdb"
DB_PASSWORD="userspass"
RETENTION_DAYS=7

# Создаём папки для бекапов и логов, если их ещё нет
mkdir -p "$BACKUP_DIR"
mkdir -p "$LOGS_DIR"

# Генерируем имя файла с текущей датой и временем
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
BACKUP_FILE="$BACKUP_DIR/backup_${TIMESTAMP}.sql"
LOG_FILE="$LOGS_DIR/backup.log"

# Функция для логирования
log() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] $message" | tee -a "$LOG_FILE"
}

log "═══════════════════════════════════════════════════════"
log "Начинается создание бекапа базы данных"
log "═══════════════════════════════════════════════════════"

# Проверяем, запущен ли контейнер Docker
if ! docker ps | grep -q "$CONTAINER_NAME"; then
    log "✗ ОШИБКА: Контейнер $CONTAINER_NAME не запущен!"
    exit 1
fi

log "✓ Контейнер $CONTAINER_NAME найден и активен"

# Выполняем pg_dump через Docker контейнер
log "Выполняю дамп базы данных..."

if docker exec -e PGPASSWORD="$DB_PASSWORD" "$CONTAINER_NAME" pg_dump \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  --no-password \
  > "$BACKUP_FILE" 2>> "$LOG_FILE"; then

    # Проверяем размер файла и его содержимое
    if [ -s "$BACKUP_FILE" ]; then
        SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
        LINES=$(wc -l < "$BACKUP_FILE")
        log "✓ Бекап успешно создан"
        log "  Файл: $BACKUP_FILE"
        log "  Размер: $SIZE"
        log "  Строк SQL: $LINES"
    else
        log "✗ ОШИБКА: Бекап создан, но файл пуст!"
        rm -f "$BACKUP_FILE"
        exit 1
    fi
else
    log "✗ ОШИБКА: не удалось создать дамп базы данных"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# Удаляем старые бекапы (старше RETENTION_DAYS дней)
log "Удаление старых бекапов (старше $RETENTION_DAYS дней)..."
OLD_BACKUPS=$(find "$BACKUP_DIR" -type f -name "backup_*.sql" -mtime +$RETENTION_DAYS 2>/dev/null | wc -l)

if [ "$OLD_BACKUPS" -gt 0 ]; then
    find "$BACKUP_DIR" -type f -name "backup_*.sql" -mtime +$RETENTION_DAYS -delete
    log "✓ Удалено старых бекапов: $OLD_BACKUPS"
else
    log "ℹ Старых бекапов для удаления не найдено"
fi

# Показываем статистику
TOTAL_BACKUPS=$(find "$BACKUP_DIR" -type f -name "backup_*.sql" | wc -l)
TOTAL_SIZE=$(du -sh "$BACKUP_DIR" | cut -f1)

log "═══════════════════════════════════════════════════════"
log "✓ УСПЕШНО: Бекап завершен"
log "  Всего бекапов: $TOTAL_BACKUPS"
log "  Общий размер: $TOTAL_SIZE"
log "═══════════════════════════════════════════════════════"
log ""
EOF

chmod +x "$BACKUP_SCRIPT"
echo "✓ Скрипт бекапа создан: $BACKUP_SCRIPT"
echo ""

# Шаг 3: Тестируем скрипт
echo "🧪 Тестирование скрипта..."
if bash "$BACKUP_SCRIPT"; then
    echo "✓ Скрипт работает корректно!"
    echo ""
else
    echo "✗ ОШИБКА: Скрипт завершился с ошибкой"
    echo "Проверьте логи в $LOGS_DIR/backup.log"
    exit 1
fi

# Шаг 4: Настройка cron
echo "⏰ Настройка cron задачи..."

# Определяем интервал (по умолчанию ежедневно в 2:00)
read -p "Во сколько часов создавать бекап? (по умолчанию 2): " BACKUP_HOUR
BACKUP_HOUR=${BACKUP_HOUR:-2}

# Формируем cron выражение (ежедневно в указанный час)
CRON_EXPR="0 $BACKUP_HOUR * * * cd $PROJECT_DIR && bash $BACKUP_SCRIPT >> $LOGS_DIR/backup.log 2>&1"

# Получаем текущий crontab (если существует)
CURRENT_CRON=$(crontab -l 2>/dev/null || echo "")

# Проверяем, не добавлена ли уже такая задача
if echo "$CURRENT_CRON" | grep -q "$BACKUP_SCRIPT"; then
    echo "ℹ Cron задача для бекапов уже существует"
    echo "  Текущая задача:"
    echo "$CURRENT_CRON" | grep "$BACKUP_SCRIPT" | sed 's/^/    /'
else
    # Добавляем новую cron задачу
    (echo "$CURRENT_CRON"; echo "$CRON_EXPR") | crontab -
    echo "✓ Cron задача добавлена"
    echo "  Время: $BACKUP_HOUR:00 каждый день"
    echo "  Команда: $CRON_EXPR"
fi

echo ""
echo "================================"
echo "✓ НАСТРОЙКА ЗАВЕРШЕНА"
echo "================================"
echo ""
echo "📊 Информация о настройке:"
echo "  • Папка бекапов: $BACKUP_DIR"
echo "  • Папка логов: $LOGS_DIR"
echo "  • Скрипт бекапа: $BACKUP_SCRIPT"
echo ""
echo "🔧 Полезные команды:"
echo "  • Проверить бекапы: ls -lah $BACKUP_DIR/"
echo "  • Посмотреть логи: cat $LOGS_DIR/backup.log"
echo "  • Запустить вручную: bash $BACKUP_SCRIPT"
echo "  • Проверить cron: crontab -l"
echo "  • Отредактировать cron: crontab -e"
echo ""
echo "💡 Совет: Рекомендуется периодически проверять логи на ошибки"
echo ""
