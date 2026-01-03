--удаляем таблицы, если они существуют
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS users;

CREATE TABLE users (
    user_id VARCHAR(20) NOT NULL UNIQUE, -- ID телеграмма 8-10 символов, но берем про запас
    balance INTEGER,
    trial BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

        --ссылается на user_id в таблице users
        REFERENCES users(user_id)

        --при удалении пользователя удалить все его транзакции
        ON DELETE CASCADE
        ON UPDATE CASCADE
);