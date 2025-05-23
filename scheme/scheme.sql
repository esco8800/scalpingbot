CREATE TABLE IF NOT EXISTS users
(
    telegram_id      TEXT PRIMARY KEY,
    username         TEXT,
    profit_percent   REAL,
    order_size       REAL,
    base_buy_timeout INTEGER,
    api_key          TEXT,
    secret_key       TEXT,
    symbol           TEXT,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)