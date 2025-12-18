#!/bin/bash

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Автоматический релиз${NC}"
echo "=================================="

# Определяем текущую ветку
current_branch=$(git rev-parse --abbrev-ref HEAD)
if [[ -z "$current_branch" ]]; then
    echo -e "${RED}❌ Не удалось определить текущую ветку${NC}"
    exit 1
fi
echo -e "${GREEN}📋 Текущая ветка: $current_branch${NC}"

# Функция для получения последнего тега
get_latest_tag() {
    # Получаем все теги, начинающиеся с 'v' и содержащие цифры
    local tags=$(git tag -l "v*" | grep -E '^v[0-9]+\.[0-9]+(\.[0-9]+)?$' | sort -V)
    
    if [[ -z "$tags" ]]; then
        echo ""
        return
    fi
    
    # Возвращаем последний тег (самую большую версию)
    echo "$tags" | tail -n1
}

# Функция для увеличения версии
increment_version() {
    local version=$1
    
    # Проверяем формат vX.Y (основной формат)
    if [[ $version =~ ^v([0-9]+)\.([0-9]+)$ ]]; then
        local major=${BASH_REMATCH[1]}
        local minor=${BASH_REMATCH[2]}
        local new_minor=$((minor + 1))
        echo "v${major}.${new_minor}"
    # Проверяем формат vX.Y.Z (с патчем)
    elif [[ $version =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        local major=${BASH_REMATCH[1]}
        local minor=${BASH_REMATCH[2]}
        local patch=${BASH_REMATCH[3]}
        local new_patch=$((patch + 1))
        echo "v${major}.${minor}.${new_patch}"
    # Если формат не распознан, начинаем с v0.1
    else
        echo "v0.1"
    fi
}

# Проверяем, есть ли изменения для коммита
if [[ -z $(git status --porcelain) ]]; then
    echo -e "${YELLOW}⚠️  Нет изменений для коммита${NC}"
    read -p "Продолжить создание тега без коммита? [y/N]: " continue_without_commit
    if [[ $continue_without_commit != "y" && $continue_without_commit != "Y" ]]; then
        echo -e "${RED}❌ Релиз отменен${NC}"
        exit 1
    fi
    skip_commit=true
else
    skip_commit=false
fi

# Получаем текущий максимальный тег
current_tag=$(get_latest_tag)
if [[ -z $current_tag ]]; then
    new_tag="v0.1"
    echo -e "${YELLOW}📋 Текущих тегов не найдено, начинаем с $new_tag${NC}"
else
    new_tag=$(increment_version $current_tag)
    if [[ $? -ne 0 || -z $new_tag ]]; then
        echo -e "${RED}❌ Ошибка при определении новой версии${NC}"
        echo -e "${YELLOW}💡 Попробуйте создать тег вручную или проверьте формат существующих тегов${NC}"
        exit 1
    fi
    echo -e "${GREEN}📋 Текущий тег: $current_tag${NC}"
    echo -e "${GREEN}📋 Следующий тег: $new_tag${NC}"
fi

# Проверяем, что новый тег не существует
if git tag -l | grep -q "^$new_tag$"; then
    echo -e "${RED}❌ Тег $new_tag уже существует!${NC}"
    echo -e "${YELLOW}💡 Возможно, произошла ошибка в логике инкремента версии${NC}"
    exit 1
fi

echo -e "${BLUE}🏷️  Новый тег: $new_tag${NC}"

# Запрашиваем сообщение коммита и тега
if [[ $skip_commit == false ]]; then
    echo ""
    read -p "💬 Введите сообщение коммита: " commit_message
    if [[ -z "$commit_message" ]]; then
        commit_message="Release $new_tag"
    fi
fi

echo ""
read -p "🏷️  Введите сообщение для тега [$new_tag]: " tag_message
if [[ -z "$tag_message" ]]; then
    tag_message="Release $new_tag"
fi

# Подтверждение
echo ""
echo -e "${YELLOW}📝 Сводка релиза:${NC}"
echo "  Ветка: $current_branch"
if [[ -n $current_tag ]]; then
    echo "  Предыдущий тег: $current_tag"
fi
echo "  Новый тег: $new_tag"
if [[ $skip_commit == false ]]; then
    echo "  Сообщение коммита: $commit_message"
fi
echo "  Сообщение тега: $tag_message"
echo ""

read -p "🚀 Продолжить релиз? [Y/n]: " confirm
if [[ $confirm == "n" || $confirm == "N" ]]; then
    echo -e "${RED}❌ Релиз отменен${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}🔄 Выполняем релиз...${NC}"

# Выполняем git команды
if [[ $skip_commit == false ]]; then
    echo -e "${YELLOW}📦 Добавляем файлы...${NC}"
    git add .
    
    echo -e "${YELLOW}💾 Создаем коммит...${NC}"
    git commit -m "$commit_message"
    
    if [[ $? -ne 0 ]]; then
        echo -e "${RED}❌ Ошибка при создании коммита${NC}"
        exit 1
    fi
fi

echo -e "${YELLOW}🏷️  Создаем тег...${NC}"
git tag -a "$new_tag" -m "$tag_message"

if [[ $? -ne 0 ]]; then
    echo -e "${RED}❌ Ошибка при создании тега${NC}"
    exit 1
fi

echo -e "${YELLOW}📤 Отправляем тег...${NC}"
git push origin "$new_tag"

if [[ $? -ne 0 ]]; then
    echo -e "${RED}❌ Ошибка при отправке тега${NC}"
    exit 1
fi

# Отправляем изменения в текущую ветку
echo -e "${YELLOW}📤 Отправляем изменения в $current_branch...${NC}"
git push origin "$current_branch"

if [[ $? -ne 0 ]]; then
    echo -e "${RED}❌ Ошибка при отправке в $current_branch${NC}"
    echo -e "${YELLOW}⚠️  Возможно, в удаленном репозитории есть изменения, которых нет локально${NC}"
    echo -e "${YELLOW}⚠️  Это может привести к перезаписи данных на GitHub${NC}"
    echo ""
    read -p "🚨 Вы уверены, что хотите принудительно перезаписать данные на GitHub? [y/N]: " force_push
    if [[ $force_push == "y" || $force_push == "Y" ]]; then
        echo -e "${YELLOW}🔄 Принудительно отправляем изменения...${NC}"
        git push --force origin "$current_branch"
        
        if [[ $? -ne 0 ]]; then
            echo -e "${RED}❌ Ошибка при принудительной отправке в $current_branch${NC}"
            exit 1
        fi
        echo -e "${GREEN}✅ Изменения принудительно отправлены в $current_branch${NC}"
    else
        echo -e "${YELLOW}⚠️  Отправка в $current_branch отменена${NC}"
        echo -e "${YELLOW}💡 Рекомендуется выполнить 'git pull origin $current_branch' для синхронизации${NC}"
    fi
fi

# Создаем GitHub Release
echo ""
echo -e "${YELLOW}🚀 Создаем GitHub Release для $new_tag...${NC}"

# Проверяем, установлен ли gh CLI
if ! command -v gh &> /dev/null; then
    echo -e "${RED}❌ GitHub CLI (gh) не установлен${NC}"
    echo -e "${YELLOW}💡 Установите GitHub CLI для автоматического создания релизов${NC}"
    echo -e "${YELLOW}   Ubuntu/Debian: sudo apt install gh${NC}"
    echo -e "${YELLOW}   Или скачайте с: https://cli.github.com/${NC}"
else
    # Проверяем авторизацию в GitHub
    if ! gh auth status &> /dev/null; then
        echo -e "${RED}❌ Не авторизован в GitHub CLI${NC}"
        echo -e "${YELLOW}💡 Выполните: gh auth login${NC}"
    else
        # Создаем релиз
        gh release create "$new_tag" --title "$new_tag" --notes "$tag_message"
        
        if [[ $? -eq 0 ]]; then
            echo -e "${GREEN}✅ GitHub Release $new_tag успешно создан!${NC}"
        else
            echo -e "${RED}❌ Ошибка при создании GitHub Release${NC}"
        fi
    fi
fi

echo ""
echo -e "${GREEN}✅ Релиз $new_tag успешно создан!${NC}"
echo -e "${GREEN}🎉 Все команды выполнены успешно${NC}"