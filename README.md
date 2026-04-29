# Установка и настройка
## Docker
```bash
curl -fsSL https://get.docker.com | sh
```

## .env 
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

# Если используете SOCKS5, то тут можно указать адрес
# SOCKS5
# SOCKS5_HOST=
# SOCKS5_PORT=

PRICE_PER_MONTH=100 # стоимость подписки за один месяц
DEVICE_LIMIT=2 # указать количество которое указан в дефолте remnawave
TRAFFIC_LIMIT=250 # количесвто гигабайт клиенту на месяц

# Logger. Можно ставить info, debug, warn, error.
LOGGER_LEVEL=info

# DB
DATABASE_URL=postgres://user:userspass@localhost:5432/usersdb?sslmode=disable
```

## Vault

*Если хотите спрятать секреты от root — читайте секреты из Vault, а не из `.env`*

Vault **отключён по умолчанию** — все секреты берутся из `.env`.  
Чтобы включить, добавь в `.env`:

```env
VAULT=enable
VAULT_ADDRESS=http://vault:8200
VAULT_ROLE_ID=
```

### Первый запуск (один раз)

```bash
make dcm
```

**Инициализация Vault** - сохрани ВСЁ что выведет:
```bash
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 vault vault operator init
```

**Распечатай Vault** (введи 3 из 5 ключей):
```bash
make vault-unseal
```

**Включи KV v2 и запиши секреты:**
```bash
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=<RootToken> vault vault secrets enable -path=secret kv-v2
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=<RootToken> vault vault kv put secret/proxymaster \
  DATABASE_URL="postgres://..." \
  TELEGRAM_TOKEN="..." \
  ...
```

**Настрой AppRole:**
Он выдаст `role-id` и `secret-id`, их нужно сохранить

```bash
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 vault sh -c \
  'echo '\''path "secret/data/proxymaster/*" { capabilities = ["read", "list"] }'\'' | vault policy write proxymaster -'
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=<RootToken> vault vault auth enable approle
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=<RootToken> vault vault write auth/approle/role/proxymaster \
  secret_id_ttl=0 token_ttl=1h token_policies=proxymaster
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=<RootToken> vault vault read auth/approle/role/proxymaster/role-id
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=<RootToken> vault vault write -f auth/approle/role/proxymaster/secret-id
```

`RoleID` запиши в `.env`, SecretID не храни на сервере - передавай при запуске вот так:
```bash
VAULT_SECRET_ID=xxx make dcm
```

**.env**:
```env
VAULT=enable
VAULT_ADDRESS=http://vault:8200
VAULT_ROLE_ID=...
```

### После каждого ребута сервера

Vault снова sealed. Распечатай тремя ключами:

```bash
make vault-unseal
```

Запусти приложение, передав SecretID:

```bash
VAULT_SECRET_ID=xxx make dcm
```

