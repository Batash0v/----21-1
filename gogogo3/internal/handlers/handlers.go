// internal/handlers/handlers.go
package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/myserver/internal/models"
	"example.com/myserver/internal/repository"
)

// Handler хранит шаблоны и репозиторий. SQL-запросов в обработчиках нет -
// они вызывают только методы репозитория.
type Handler struct {
	repo      *repository.ExpenseRepository
	templates map[string]*template.Template
}

// PageData - данные, которые передаются в шаблоны.
type PageData struct {
	Title    string
	Expenses []models.Expense
	Total    float64
	Expense  models.Expense // для формы
	Action   string         // куда отправляется форма
	Button   string         // текст кнопки формы
}

// New загружает шаблоны. К каждой странице подключается общий layout.html.
func New(repo *repository.ExpenseRepository) (*Handler, error) {
	h := &Handler{
		repo:      repo,
		templates: make(map[string]*template.Template),
	}

	pages := []string{"list.html", "form.html", "about.html"}
	for _, page := range pages {
		t, err := template.ParseFiles("web/templates/layout.html", "web/templates/"+page)
		if err != nil {
			return nil, err
		}
		h.templates[page] = t
	}
	return h, nil
}

// Routes регистрирует маршруты.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", h.List)
	mux.HandleFunc("GET /expenses/new", h.NewForm)
	mux.HandleFunc("POST /expenses", h.Create)
	mux.HandleFunc("GET /expenses/{id}/edit", h.EditForm)
	mux.HandleFunc("POST /expenses/{id}/edit", h.Update)
	mux.HandleFunc("POST /expenses/{id}/delete", h.Delete)
	mux.HandleFunc("GET /about", h.About)
	mux.HandleFunc("/ping", h.Ping)

	// раздача CSS
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	return mux
}

// render собирает страницу из layout и шаблона страницы.
func (h *Handler) render(w http.ResponseWriter, page string, data PageData) {
	t, ok := h.templates[page]
	if !ok {
		http.Error(w, "шаблон не найден", http.StatusInternalServerError)
		return
	}

	// сначала рендерим в буфер, чтобы при ошибке не отправить пользователю половину страницы
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		h.serverError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// serverError пишет ошибку в лог и отвечает пользователю кодом 500.
func (h *Handler) serverError(w http.ResponseWriter, err error) {
	log.Println("ошибка:", err)
	http.Error(w, "внутренняя ошибка сервера", http.StatusInternalServerError)
}

// List показывает список трат.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	expenses, err := h.repo.GetAll()
	if err != nil {
		h.serverError(w, err)
		return
	}

	total := 0.0
	for _, e := range expenses {
		total += e.Amount
	}

	h.render(w, "list.html", PageData{
		Title:    "Список трат",
		Expenses: expenses,
		Total:    total,
	})
}

// NewForm показывает форму добавления траты.
func (h *Handler) NewForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, "form.html", PageData{
		Title:   "Новая трата",
		Expense: models.Expense{Date: time.Now().Format("2006-01-02")},
		Action:  "/expenses",
		Button:  "Добавить",
	})
}

// Create принимает данные формы, сохраняет трату в базу и делает redirect.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	e, err := readExpenseForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := h.repo.Create(e); err != nil {
		h.serverError(w, err)
		return
	}

	// Post/Redirect/Get: после POST отправляем пользователя на GET-страницу,
	// чтобы при обновлении страницы форма не отправлялась повторно.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// EditForm показывает форму редактирования с уже заполненными полями.
func (h *Handler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "некорректный id", http.StatusBadRequest)
		return
	}

	e, err := h.repo.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.render(w, "form.html", PageData{
		Title:   "Редактирование траты",
		Expense: e,
		Action:  fmt.Sprintf("/expenses/%d/edit", e.ID),
		Button:  "Сохранить",
	})
}

// Update сохраняет изменения траты.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "некорректный id", http.StatusBadRequest)
		return
	}

	e, err := readExpenseForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	e.ID = id

	err = h.repo.Update(e)
	if errors.Is(err, repository.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Delete удаляет трату. Подтверждение показывает браузер (см. list.html).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "некорректный id", http.StatusBadRequest)
		return
	}

	err = h.repo.Delete(id)
	if errors.Is(err, repository.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// About - страница с описанием проекта.
func (h *Handler) About(w http.ResponseWriter, r *http.Request) {
	h.render(w, "about.html", PageData{Title: "О проекте"})
}

// Ping возвращает pong только на GET, на остальные методы - 405.
func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "pong")
}

// readExpenseForm достаёт трату из данных формы и проверяет их.
func readExpenseForm(r *http.Request) (models.Expense, error) {
	var e models.Expense

	if err := r.ParseForm(); err != nil {
		return e, errors.New("не удалось разобрать форму")
	}

	// принимаем и запятую, и точку: 12,50 или 12.50
	amountStr := strings.TrimSpace(r.FormValue("amount"))
	amountStr = strings.Replace(amountStr, ",", ".", 1)
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return e, errors.New("сумма должна быть положительным числом")
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if description == "" {
		return e, errors.New("описание не может быть пустым")
	}

	date := r.FormValue("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return e, errors.New("дата должна быть в формате ГГГГ-ММ-ДД")
	}

	e.Amount = amount
	e.Description = description
	e.Date = date
	return e, nil
}
