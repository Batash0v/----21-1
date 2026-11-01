// internal/handlers/auth.go
package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"example.com/myserver/internal/repository"
)

// RegisterForm показывает форму регистрации.
func (h *Handler) RegisterForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := userFromContext(r.Context()); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, r, "register.html", PageData{Title: "Регистрация"})
}

// Register создаёт нового пользователя.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "не удалось разобрать форму", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	password2 := r.FormValue("password2")

	data := PageData{Title: "Регистрация", FormEmail: email}

	addr, err := mail.ParseAddress(email)
	switch {
	case err != nil || addr.Address != email:
		data.Error = "Введите корректный email"
	case utf8.RuneCountInString(password) < 6:
		data.Error = "Пароль должен быть не короче 6 символов"
	case len(password) > 72: // bcrypt принимает не больше 72 байт
		data.Error = "Пароль слишком длинный (максимум 72 байта)"
	case password != password2:
		data.Error = "Пароли не совпадают"
	}
	if data.Error != "" {
		h.renderStatus(w, r, http.StatusBadRequest, "register.html", data)
		return
	}

	// В базу попадает только хэш. Из хэша пароль восстановить нельзя.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.serverError(w, err)
		return
	}

	_, err = h.users.Create(email, string(hash))
	if errors.Is(err, repository.ErrEmailTaken) {
		data.Error = "Пользователь с таким email уже зарегистрирован"
		h.renderStatus(w, r, http.StatusConflict, "register.html", data)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/login?registered=1", http.StatusSeeOther)
}

// LoginForm показывает форму входа.
func (h *Handler) LoginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := userFromContext(r.Context()); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := PageData{Title: "Вход"}
	if r.URL.Query().Get("registered") == "1" {
		data.Notice = "Регистрация прошла успешно. Теперь можно войти."
	}
	h.render(w, r, "login.html", data)
}

// Login проверяет пароль, создаёт сессию и ставит cookie.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "не удалось разобрать форму", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	user, err := h.users.GetByEmail(email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		h.serverError(w, err)
		return
	}

	// Одно и то же сообщение и для неверного email, и для неверного пароля,
	// чтобы нельзя было узнать, какие email зарегистрированы.
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		h.renderStatus(w, r, http.StatusUnauthorized, "login.html", PageData{
			Title:     "Вход",
			FormEmail: email,
			Error:     "Неверный email или пароль",
		})
		return
	}

	// заодно чистим просроченные сессии
	if err := h.sessions.DeleteExpired(); err != nil {
		log.Println("не удалось удалить старые сессии:", err)
	}

	sessionID, expires, err := h.sessions.Create(user.ID, sessionTTL)
	if err != nil {
		h.serverError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionID,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,                 // JavaScript на странице не сможет прочитать cookie
		SameSite: http.SameSiteLaxMode, // cookie не отправляется с чужих сайтов при POST
		// Secure: true нужно включить при работе по HTTPS; на localhost по HTTP оно бы сломало вход
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout удаляет сессию из базы и стирает cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := h.sessions.Delete(cookie.Value); err != nil {
			log.Println("не удалось удалить сессию:", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1, // браузер сразу удаляет cookie
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
