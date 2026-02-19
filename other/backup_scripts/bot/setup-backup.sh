#!/bin/bash

# Скрипт для установки и настройки автоматических бекапов БД из Docker контейнера
# Выполняет все необходимые действия: создание папок и настройку cron
# Создаёт всё в директории other/ рядом с этим скриптом

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Определяем путь к директории other/ (где находится этот скрипт)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="$SCRIPT_DIR/backups"
LOGS_DIR="$SCRIPT_DIR/logs"
BACKUP_SCRIPT="$SCRIPT_DIR/backup.sh"
CONTAINER_NAME="users_postgres"
DB_USER="user"
DB_NAME="usersdb"
DB_PASSWORD="userspass"
RETENTION_DAYS=7
BACKUP_HOUR=2

echo -e "${YELLOW}🚀 Начинается установка системы автоматических бекапов...${NC}"
echo ""
echo -e "${YELLOW}Директория установки: $SCRIPT_DIR${NC}"
echo ""

# Шаг 1: Создание папок
echo -e "${YELLOW}[1/3] Создание необходимых папок...${NC}"
mkdir -p "$BACKUP_DIR"
mkdir -p "$LOGS_DIR"
echo -e "${GREEN}✓ Папки созданы:${NC}"
echo "  • $BACKUP_DIR"
echo "  • $LOGS_DIR"
echo ""

# Шаг 2: Проверка скрипта бекапа
echo -e "${YELLOW}[2/3] Проверка скрипта бекапа...${NC}"
if [ -f "$BACKUP_SCRIPT" ]; then
  chmod +x "$BACKUP_SCRIPT"
  echo -e "${GREEN}✓ Скрипт бекапа найден и прав доступа установлены: $BACKUP_SCRIPT${NC}"
  echo ""
else
  echo -e "${RED}✗ ОШИБКА: скрипт бекапа не найден по пути: $BACKUP_SCRIPT${NC}"
  exit 1
fi

# Шаг 3: Настройка cron
echo -e "${YELLOW}[3/3] Настройка cron задачи...${NC}"

CRON_COMMAND="0 $BACKUP_HOUR * * * bash $BACKUP_SCRIPT >> $LOGS_DIR/backup.log 2>&1"
CRON_COMMENT="# ProxyMaster Database Backup - $BACKUP_SCRIPT"

# Получаем текущий crontab (или пустой, если его нет)
CURRENT_CRON=$(crontab -l 2>/dev/null || echo "")

# Проверяем, есть ли уже наша задача
if echo "$CURRENT_CRON" | grep -q "backup.sh"; then
  echo -e "${YELLOW}⚠ Cron задача для backup.sh уже существует${NC}"
  echo "Текущая задача:"
  echo "$CURRENT_CRON" | grep "backup.sh" | sed 's/^/  /'
  echo ""
  echo -e "${YELLOW}Хочешь заменить её? (y/n)${NC}"
  read -r REPLACE
  if [ "$REPLACE" = "y" ]; then
    # Удаляем старую задачу
    UPDATED_CRON=$(echo "$CURRENT_CRON" | grep -v "backup.sh")
    echo "$UPDATED_CRON" | crontab -
    # Добавляем новую задачу
    (echo "$UPDATED_CRON"; echo "$CRON_COMMENT"; echo "$CRON_COMMAND") | crontab -
    echo -e "${GREEN}✓ Cron задача обновлена${NC}"
  else
    echo "Пропуск обновления cron"
  fi
else
  # Добавляем новую задачу
  (echo "$CURRENT_CRON"; echo "$CRON_COMMENT"; echo "$CRON_COMMAND") | crontab -
  echo -e "${GREEN}✓ Cron задача добавлена${NC}"
fi

echo ""
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ УСТАНОВКА ЗАВЕРШЕНА УСПЕШНО!${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo "📋 Сводка:"
echo "  • Папка бекапов: $BACKUP_DIR"
echo "  • Папка логов: $LOGS_DIR"
echo "  • Скрипт бекапа: $BACKUP_SCRIPT"
echo "  • Время запуска: $BACKUP_HOUR:00 каждый день"
echo "  • Хранение бекапов: $RETENTION_DAYS дней"
echo ""
echo "📝 Полезные команды:"
echo "  • Просмотр логов: tail -f $LOGS_DIR/backup.log"
echo "  • Список бекапов: ls -lh $BACKUP_DIR"
echo "  • Ручной бекап: bash $BACKUP_SCRIPT"
echo "  • Просмотр cron: crontab -l"
echo ""
