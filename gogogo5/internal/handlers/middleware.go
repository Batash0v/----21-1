// internal/handlers/middleware.go
package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/myserver/internal/models"
	"example.com/myserver/internal/repository"
)

const (
	sessionCookie = "session_id"
	sessionTTL    = 24 * time.Hour // сколько живёт сессия после входа
)

// contextKey - отдельный тип для ключа в context, чтобы ключ не совпал с ключами из других пакетов.
type contextKey string

const userKey contextKey = "user"

// userFromContext достаёт из контекста запроса пользователя, которого туда положил loadUser.
func userFromContext(ctx context.Context) (models.User, bool) {
	user, ok := ctx.Value(userKey).(models.User)
	return user, ok
}

// currentUserID возвращает id вошедшего пользователя (0, если никто не вошёл).
func currentUserID(r *http.Request) int {
	user, _ := userFromContext(r.Context())
	return user.ID
}

// loadUser - middleware для всех запросов. Смотрит cookie с id сессии и, если сессия
// действует, кладёт пользователя в контекст запроса. Никого никуда не перенаправляет:
// гость просто проходит дальше без пользователя в контексте.
func (h *Handler) loadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			// cookie нет - значит, гость
			next.ServeHTTP(w, r)
			return
		}

		user, err := h.sessions.GetUser(cookie.Value)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			h.serverError(w, err)
			return
		}
		if err == nil {
			ctx := context.WithValue(r.Context(), userKey, user)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// requireAuth - middleware для страниц, закрытых от гостей: если пользователя в контексте
// нет (не входил или сессия истекла), отправляем на страницу входа.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := userFromContext(r.Context()); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
