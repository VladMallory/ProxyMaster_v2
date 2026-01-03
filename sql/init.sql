DROP TABLE IF EXISTS users;

CREATE TABLE users (
    id VARCHAR(20) NOT NULL UNIQUE, -- ID телеграмма 8-10 символов, но берем про запас
    balance INTEGER,
    trial BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE transactions (
    id VARCHAR(36) PRIMARY KEY, -- UUID транзакции
    user_id VARCHAR(20) NOT NULL, -- ID пользователя
    amount INTEGER NOT NULL, -- Сумма пополнения
    status VARCHAR(20) NOT NULL, -- Статус: pending, success, failed
    provider VARCHAR(50) NOT NULL, -- Провайдер платежа
    external_id VARCHAR(100), -- ID транзакции в платежной системе
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);