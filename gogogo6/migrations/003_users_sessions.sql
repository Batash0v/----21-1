-- migrations/003_users_sessions.sql
-- Пользователи, сессии и привязка трат к пользователю.

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL
);

-- expires_at - время окончания сессии в секундах Unix
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
);

-- Траты, созданные до появления пользователей, остаются с user_id = NULL
-- и никому не показываются.
ALTER TABLE expenses ADD COLUMN user_id INTEGER REFERENCES users(id);
