// internal/repository/session_repository.go
package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"example.com/myserver/internal/models"
)

// SessionRepository - SQL-запросы для таблицы sessions.
// Сессии хранятся в базе, поэтому переживают перезапуск сервера.
type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// newSessionID создаёт случайный идентификатор сессии, который нельзя угадать.
func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create создаёт сессию на время ttl и возвращает её id и время окончания.
func (r *SessionRepository) Create(userID int, ttl time.Duration) (string, time.Time, error) {
	id, err := newSessionID()
	if err != nil {
		return "", time.Time{}, err
	}

	expires := time.Now().Add(ttl)
	_, err = r.db.Exec(
		"INSERT INTO sessions (session_id, user_id, expires_at) VALUES (?, ?, ?)",
		id, userID, expires.Unix(),
	)
	if err != nil {
		return "", time.Time{}, err
	}

	return id, expires, nil
}

// GetUser возвращает владельца сессии. Если сессии нет или она истекла - ErrNotFound.
func (r *SessionRepository) GetUser(sessionID string) (models.User, error) {
	var u models.User

	// срок действия проверяется прямо в запросе: expires_at > текущее время
	err := r.db.QueryRow(
		`SELECT u.id, u.email
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.session_id = ? AND s.expires_at > ?`,
		sessionID, time.Now().Unix(),
	).Scan(&u.ID, &u.Email)

	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// Delete удаляет сессию (выход из системы).
func (r *SessionRepository) Delete(sessionID string) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE session_id = ?", sessionID)
	return err
}

// DeleteExpired удаляет все просроченные сессии, чтобы таблица не росла.
func (r *SessionRepository) DeleteExpired() error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE expires_at <= ?", time.Now().Unix())
	return err
}
