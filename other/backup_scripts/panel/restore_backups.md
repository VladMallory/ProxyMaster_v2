# Создаем бекап
## Узнаем параметры
```bash
docker exec -it remnawave-db env | grep POSTGRES
```

Нужно записать:
- **Контейнер**: remnawave-db
- **Пользователь**: postgres
- **Пароль**: postgres
- **Имя БД**: postgres

## Создаем бекап внутри контейнера
```bash
docker exec remnawave-db pg_dump \
  -U postgres \
  -F c \
  -b \
  -v \
  -f /tmp/postgres_full_backup.dump \
  postgres
```

Если просит пароль, ввести: *postgres*

## Копируем бекап на хост
```bash
docker cp remnawave-db:/tmp/postgres_full_backup.dump \
  /root/postgres_backup_$(date +%F_%H-%M).dump
```

---
# Востанавливаем из бекапа
## Копируем бекап в контейнер
```bash
docker cp postgres_backup_2025-12-19_03-00.dump remnawave-db:/tmp/restore.dump
```

## ЕСЛИ БАЗА УЖЕ СУЩЕСТВУЕТ, ВВЕСТИ ЭТО. А ОНА ТОЧНО СУЩЕСТВУЕТ
```bash
docker exec -it remnawave-db pg_restore \
  -U postgres \
  -d postgres \
  --clean \
  --if-exists \
  -v \
  /tmp/restore.dump
```

## Востанавливаем
```bash
docker exec -it remnawave-db pg_restore \
  -U postgres \
  -d postgres \
  -v \
  /tmp/restore.dump
```
