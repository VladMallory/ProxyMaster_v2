-- +goose Up
-- Модель «каждое устройство отдельно»: одна строка devices = одно оплаченное
-- место со своим сроком. hwid пуст, пока устройство не подключилось.
CREATE TABLE devices (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username   TEXT        NOT NULL,
    hwid       TEXT,
    label      TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Уникальность (username, hwid) только для привязанных мест: свободных строк
-- с hwid IS NULL может быть сколько угодно, поэтому индекс частичный.
CREATE UNIQUE INDEX devices_username_hwid_uq
    ON devices (username, hwid) WHERE hwid IS NOT NULL;

CREATE INDEX devices_username_idx ON devices (username);

-- Партиальный индекс под выборку истекающих — по нему ходит воркер отзыва.
CREATE INDEX devices_to_revoke_idx
    ON devices (expires_at) WHERE revoked_at IS NULL;

-- Дедупликация оплат: одна строка на платёж, вставляется в транзакции покупки,
-- поэтому повторный вебхук не создаст места дважды.
CREATE TABLE processed_payments (
    payment_id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_payments;
DROP TABLE devices;
