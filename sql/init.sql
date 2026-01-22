-- init.sql поднимает базовую схему Postgres для ProxyMaster_v2
-- удаляем таблицы, если они существуют
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS device_addons;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS squads;

CREATE TABLE users (
    id VARCHAR(20) PRIMARY KEY, -- ID телеграмма 8-10 символов, но берем про запас
    balance INTEGER NOT NULL DEFAULT 0,
    trial BOOLEAN NOT NULL DEFAULT FALSE,
    -- Кол-во купленных "доп. устройств" (для отображения, быстрой аналитики)
    -- Фактическое списание и расписание хранится в таблице device_addons
    extra_devices_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE device_addons (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(20) NOT NULL,
    next_charge_at TIMESTAMP NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_device_addons_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);

CREATE TABLE transactions (
    uuid VARCHAR(36) PRIMARY KEY, -- uuid транзакции
    user_id VARCHAR(20) NOT NULL, -- ID пользователя -- внешний ключ на таблицу users
    amount INTEGER NOT NULL, -- Сумма пополнения
    status VARCHAR(20) NOT NULL, -- Статус: pending, success, failed
    provider VARCHAR(50) NOT NULL, -- Провайдер платежа
    external_id VARCHAR(100), -- ID транзакции в платежной системе
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    --внешний ключ на таблицу users
    CONSTRAINT fk_transactions_user

        --ключ user_id в таблице transactions
        FOREIGN KEY (user_id) 

        --ссылается на id в таблице users
        REFERENCES users(id)

        --при удалении пользователя удалить все его транзакции
        ON DELETE CASCADE
        ON UPDATE CASCADE
);

CREATE TABLE squads (
    title VARCHAR(100) NOT NULL,
    user_id VARCHAR(20) NOT  NULL,
    uuid VARCHAR(36) NOT NULL,
    expires_at TIMESTAMP,

    CONSTRAINT fk_squads_user

        FOREIGN KEY (user_id)
        REFERENCES users(id)

        ON DELETE CASCADE
        ON UPDATE CASCADE
);
