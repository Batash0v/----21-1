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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"example.com/myserver/internal/models"
)

// Handler хранит шаблоны и список трат (пока в памяти, без базы данных).
type Handler struct {
	templates map[string]*template.Template

	mu       sync.Mutex // защищает expenses и nextID, потому что запросы идут параллельно
	expenses []models.Expense
	nextID   int
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
func New() (*Handler, error) {
	h := &Handler{
		templates: make(map[string]*template.Template),
		nextID:    1,
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
		log.Println("ошибка шаблона:", err)
		http.Error(w, "внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// List показывает список трат.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	// копируем срез под блокировкой, чтобы спокойно с ним работать
	h.mu.Lock()
	list := make([]models.Expense, len(h.expenses))
	copy(list, h.expenses)
	h.mu.Unlock()

	// новые даты сверху
	sort.Slice(list, func(i, j int) bool {
		if list[i].Date != list[j].Date {
			return list[i].Date > list[j].Date
		}
		return list[i].ID > list[j].ID
	})

	total := 0.0
	for _, e := range list {
		total += e.Amount
	}

	h.render(w, "list.html", PageData{
		Title:    "Список трат",
		Expenses: list,
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

// Create принимает данные формы, добавляет трату в срез и делает redirect.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	e, err := readExpenseForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	e.ID = h.nextID
	h.nextID++
	h.expenses = append(h.expenses, e)
	h.mu.Unlock()

	// Post/Redirect/Get: после POST отправляем пользователя на GET-страницу,
	// чтобы при обновлении страницы форма не отправлялась повторно.
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
