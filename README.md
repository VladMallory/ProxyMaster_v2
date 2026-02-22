# Установка и настройка
### Docker
```bash
curl -fsSL https://get.docker.com | sh
```

### .env 
Создаем .env с такими параметрами
```bash
vi .env # или nano вместо vi, как удобно
```

```bash
# REMNAWAVE
REMNA_PANEL=https://panel.domen.ru/auth/login?REMNA_SECRET_TOKEN
REMNA_BASE_PANEL=https://panel.domen.ru
REMNA_SECRET_TOKEN="REMNA_SECRET_TOKEN"
REMNA_LOGIN=login
REMNA_PASS=password
REMNA_TOKEN= # берется тут remnawave/управление/настройки remnawave/API токены/Создать
REMNA_DEFOULT_GB=250
REMNA_SQUAD_UUID= # берется тут remnawave/управление/Внутренние сквады/Скопировать UUID


# telegram
TELEGRAM_TOKEN=
TELEGRAM_SUPPORT=https://t.me/
TELEGRAM_ADMIN_ID=

# Платежная система
# YOUKASSA
YOUKASSA_SHOP_ID=
YOUKASSA_SECRET_KEY=
YOUKASSA_BASE_URL=https://api.youkassa.ru
YOUKASSA_RETURN_URL=https://google.com

PRICE_PER_MONTH=100 # стоимость подписки за один месяц
DEVICE_LIMIT=2 # указать количество которое указан в дефолте remnawave
TRAFFIC_LIMIT=250 # количесвто гигабайт клиенту на месяц

# Logger. Можно ставить info, debug, warn, error.
LOGGER_LEVEL=info

# DB
DATABASE_URL=postgres://user:userspass@localhost:5432/usersdb?sslmode=disable
```

### Запуск 

#### Запустит Golang, postgreSQL, Loki, grafana в docker контейнере
```bash
make dcl
```
