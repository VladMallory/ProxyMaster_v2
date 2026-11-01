# Установка и настройка
## Docker
```bash
curl -fsSL https://get.docker.com | sh
```

## .env 
Создаем .env с такими параметрами
```bash
nano .env # или vim вместо nano, вам как удобно
```

**Вставляем в `.env` текст ниже и заполняем своими данными от панели и прочие**
```bash
# REMNAWAVE
REMNA_PANEL=https://domen.com/auth/login?123
REMNA_LOGIN=login
REMNA_PASS=password
REMNA_TOKEN= #token взять в панеле 
# нужно зайти в "Внутрении сквады" и скопировать нужный uuid
REMNA_SQUAD_UUID=

# telegram
TELEGRAM_TOKEN=
TELEGRAM_SUPPORT=
TELEGRAM_ADMIN_ID=


# Платежная система. Вписать platega или fake. Fake подходит для 
# тестирования, платежи считатаются удачными через 5 секунд
PAYMENT_PROVIDER=platega

# PLATEGA
PLATEGA_RETURN_URL=https://t.me/
PLATEGA_API_KEY=
PLATEGA_MERCHANT_ID=

PRICE_PER_MONTH=200
DEVICE_LIMIT=2
TRAFFIC_LIMIT=350
MAX_DEVICE_LIMIT=5
EXTRA_DEVICE_PRICE=10
RESET_TRAFFIC_PRICE=50
TRIAL_DAYS=30

# Автоподключение Happ/Incy. 
# WEB_ADDR адрес HTTP-страницы внутри контейнера,
# AUTO_CONNECT_SUB домен для подписки. LINK_SECRET ключ подписи, указать рандомные символы.
WEB_ADDR=:8080
AUTO_CONNECT_SUB=your-domain.ngrok-free.dev
LINK_SECRET=change_me
```

## Запуск
Дальше всегда запускать и перезагружать через
```bash
make restart
```
