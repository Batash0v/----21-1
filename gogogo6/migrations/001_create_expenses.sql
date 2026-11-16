-- migrations/001_create_expenses.sql
-- Создание таблицы трат. IF NOT EXISTS позволяет запускать скрипт при каждом старте сервера.

CREATE TABLE IF NOT EXISTS expenses (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    amount      REAL NOT NULL,
    description TEXT NOT NULL,
    date        TEXT NOT NULL
);
