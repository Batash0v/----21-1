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

// Handler хранит шаблоны и репозитории. SQL-запросов в обработчиках нет -
// они вызывают только методы репозиториев.
type Handler struct {
	repo       *repository.ExpenseRepository
	categories *repository.CategoryRepository
	users      *repository.UserRepository
	sessions   *repository.SessionRepository
	templates  map[string]*template.Template
}

// PageData - данные, которые передаются в шаблоны.
type PageData struct {
	Title      string
	Expenses   []models.Expense
	Total      float64
	Expense    models.Expense // для формы
	Categories []models.Category
	Filter     models.ExpenseFilter
	Action     string // куда отправляется форма
	Button     string // текст кнопки формы

	Email     string // email вошедшего пользователя (показывается в шапке)
	FormEmail string // email, введённый в форму входа/регистрации
	Error     string // сообщение об ошибке
	Notice    string // информационное сообщение
}

// New загружает шаблоны. К каждой странице подключается общий layout.html.
func New(
	repo *repository.ExpenseRepository,
	categories *repository.CategoryRepository,
	users *repository.UserRepository,
	sessions *repository.SessionRepository,
) (*Handler, error) {
	h := &Handler{
		repo:       repo,
		categories: categories,
		users:      users,
		sessions:   sessions,
		templates:  make(map[string]*template.Template),
	}

	pages := []string{"list.html", "form.html", "about.html", "login.html", "register.html"}
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

	// страницы, доступные всем
	mux.HandleFunc("GET /about", h.About)
	mux.HandleFunc("/ping", h.Ping)
	mux.HandleFunc("GET /register", h.RegisterForm)
	mux.HandleFunc("POST /register", h.Register)
	mux.HandleFunc("GET /login", h.LoginForm)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /logout", h.Logout)

	// страницы с тратами - только для вошедших пользователей (обёрнуты в requireAuth)
	mux.Handle("GET /{$}", requireAuth(h.List))
	mux.Handle("GET /expenses/new", requireAuth(h.NewForm))
	mux.Handle("POST /expenses", requireAuth(h.Create))
	mux.Handle("GET /expenses/{id}/edit", requireAuth(h.EditForm))
	mux.Handle("POST /expenses/{id}/edit", requireAuth(h.Update))
	mux.Handle("POST /expenses/{id}/delete", requireAuth(h.Delete))

	root := http.NewServeMux()

	// CSS отдаём без проверки сессии
	root.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Все остальные запросы проходят через loadUser, а дальше - через requireAuth (там, где он указан).
	// Порядок такой: loadUser -> requireAuth -> обработчик.
	root.Handle("/", h.loadUser(mux))

	return root
}

// render показывает страницу с кодом 200.
func (h *Handler) render(w http.ResponseWriter, r *http.Request, page string, data PageData) {
	h.renderStatus(w, r, http.StatusOK, page, data)
}

// renderStatus собирает страницу из layout и шаблона страницы и отвечает с указанным кодом.
func (h *Handler) renderStatus(w http.ResponseWriter, r *http.Request, status int, page string, data PageData) {
	t, ok := h.templates[page]
	if !ok {
		http.Error(w, "шаблон не найден", http.StatusInternalServerError)
		return
	}

	// если пользователь вошёл, в шапке показываем его email
	if user, ok := userFromContext(r.Context()); ok {
		data.Email = user.Email
	}

	// сначала рендерим в буфер, чтобы при ошибке не отправить пользователю половину страницы
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		h.serverError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// serverError пишет ошибку в лог и отвечает пользователю кодом 500.
func (h *Handler) serverError(w http.ResponseWriter, err error) {
	log.Println("ошибка:", err)
	http.Error(w, "внутренняя ошибка сервера", http.StatusInternalServerError)
}

// List показывает список трат текущего пользователя с фильтрами.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	filter := readFilter(r)

	categories, err := h.categories.GetAll()
	if err != nil {
		h.serverError(w, err)
		return
	}

	expenses, err := h.repo.GetAll(currentUserID(r), filter)
	if err != nil {
		h.serverError(w, err)
		return
	}

	total := 0.0
	for _, e := range expenses {
		total += e.Amount
	}

	h.render(w, r, "list.html", PageData{
		Title:      "Список трат",
		Expenses:   expenses,
		Total:      total,
		Categories: categories,
		Filter:     filter,
	})
}

// NewForm показывает форму добавления траты.
func (h *Handler) NewForm(w http.ResponseWriter, r *http.Request) {
	categories, err := h.categories.GetAll()
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.render(w, r, "form.html", PageData{
		Title:      "Новая трата",
		Expense:    models.Expense{Date: time.Now().Format("2006-01-02")},
		Categories: categories,
		Action:     "/expenses",
		Button:     "Добавить",
	})
}

// Create принимает данные формы, сохраняет трату в базу и делает redirect.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	e, err := readExpenseForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ok, err := h.categoryExists(e.CategoryID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !ok {
		http.Error(w, "такой категории нет", http.StatusBadRequest)
		return
	}

	// id пользователя берём из контекста (его туда положил loadUser), а не из формы -
	// иначе можно было бы подделать запрос и добавить трату от имени другого человека
	if _, err := h.repo.Create(currentUserID(r), e); err != nil {
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

	e, err := h.repo.GetByID(id, currentUserID(r))
	if errors.Is(err, repository.ErrNotFound) {
		http.NotFound(w, r) // трата не найдена или принадлежит другому пользователю
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	categories, err := h.categories.GetAll()
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.render(w, r, "form.html", PageData{
		Title:      "Редактирование траты",
		Expense:    e,
		Categories: categories,
		Action:     fmt.Sprintf("/expenses/%d/edit", e.ID),
		Button:     "Сохранить",
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

	ok, err := h.categoryExists(e.CategoryID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !ok {
		http.Error(w, "такой категории нет", http.StatusBadRequest)
		return
	}

	err = h.repo.Update(currentUserID(r), e)
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

	err = h.repo.Delete(id, currentUserID(r))
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
	h.render(w, r, "about.html", PageData{Title: "О проекте"})
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

// categoryExists проверяет, что категория с таким id есть в базе.
func (h *Handler) categoryExists(id int) (bool, error) {
	_, err := h.categories.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// readFilter достаёт фильтры из query-параметров: /?category=2&from=2026-10-01&to=2026-10-31.
// Все параметры необязательные: если параметра нет (или он некорректный), фильтр не применяется.
func readFilter(r *http.Request) models.ExpenseFilter {
	query := r.URL.Query()

	var f models.ExpenseFilter

	if id, err := strconv.Atoi(query.Get("category")); err == nil && id > 0 {
		f.CategoryID = id
	}
	f.DateFrom = validDate(query.Get("from"))
	f.DateTo = validDate(query.Get("to"))

	return f
}

// validDate возвращает строку, если это дата в формате ГГГГ-ММ-ДД, иначе пустую строку.
func validDate(s string) string {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return ""
	}
	return s
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

	categoryID, err := strconv.Atoi(r.FormValue("category_id"))
	if err != nil || categoryID <= 0 {
		return e, errors.New("выберите категорию")
	}

	e.Amount = amount
	e.Description = description
	e.Date = date
	e.CategoryID = categoryID
	return e, nil
}
