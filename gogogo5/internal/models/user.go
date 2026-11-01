// internal/models/user.go
package models

// User - зарегистрированный пользователь.
type User struct {
	ID           int
	Email        string
	PasswordHash string // bcrypt-хэш, сам пароль нигде не хранится
}
