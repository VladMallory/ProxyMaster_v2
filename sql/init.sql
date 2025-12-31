DROP TABLE IF EXISTS users;

CREATE TABLE users (
    id VARCHAR(20) NOT NULL UNIQUE, -- ID телеграмма 8-10 символов, но берем про запас
    balance INTEGER,
    trial BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);