-- migrations/002_categories.sql
-- Таблица категорий, начальные категории и связь с таблицей expenses.

CREATE TABLE IF NOT EXISTS categories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

INSERT OR IGNORE INTO categories (name) VALUES
    ('Еда'),
    ('Транспорт'),
    ('Жильё'),
    ('Развлечения'),
    ('Здоровье'),
    ('Другое');

-- Внешний ключ на categories. У трат, добавленных на прошлом этапе, категории нет (NULL).
ALTER TABLE expenses ADD COLUMN category_id INTEGER REFERENCES categories(id);
