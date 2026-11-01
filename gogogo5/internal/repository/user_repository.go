// internal/repository/user_repository.go
package repository

import (
	"database/sql"
	"errors"
	"strings"

	"example.com/myserver/internal/models"
)

// ErrEmailTaken возвращается при регистрации, если такой email уже есть.
var ErrEmailTaken = errors.New("email уже занят")

// UserRepository - SQL-запросы для таблицы users.
type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create добавляет пользователя (passwordHash - уже готовый bcrypt-хэш).
func (r *UserRepository) Create(email, passwordHash string) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO users (email, password_hash) VALUES (?, ?)",
		email, passwordHash,
	)
	if err != nil {
		// email в таблице UNIQUE, при повторе SQLite вернёт ошибку "UNIQUE constraint failed"
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, ErrEmailTaken
		}
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// GetByEmail ищет пользователя по email или возвращает ErrNotFound.
func (r *UserRepository) GetByEmail(email string) (models.User, error) {
	var u models.User

	err := r.db.QueryRow(
		"SELECT id, email, password_hash FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash)

	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}
