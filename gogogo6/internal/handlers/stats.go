// internal/handlers/stats.go
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"example.com/myserver/internal/models"
)

// categoryStatsResponse - JSON-ответ /api/stats/by-category.
type categoryStatsResponse struct {
	From  string                `json:"from"`
	To    string                `json:"to"`
	Total float64               `json:"total"`
	Items []models.CategoryStat `json:"items"`
}

// monthStatsResponse - JSON-ответ /api/stats/by-month.
type monthStatsResponse struct {
	Year   int                `json:"year"`
	Months []models.MonthStat `json:"months"`
}

// StatsPage показывает страницу с графиками. Сами данные страница подгружает через fetch.
func (h *Handler) StatsPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "stats.html", PageData{
		Title: "Статистика",
		Year:  time.Now().Year(),
	})
}

// StatsByCategory отдаёт в JSON суммы трат по категориям: /api/stats/by-category?from=...&to=...
func (h *Handler) StatsByCategory(w http.ResponseWriter, r *http.Request) {
	from, to, err := readPeriod(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// userID берём из контекста - в статистику попадают только траты вошедшего пользователя
	items, err := h.repo.SumByCategory(currentUserID(r), from, to)
	if err != nil {
		log.Println("ошибка статистики по категориям:", err)
		writeJSONError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	total := 0.0
	for _, item := range items {
		total += item.Total
	}

	writeJSON(w, http.StatusOK, categoryStatsResponse{
		From:  from,
		To:    to,
		Total: math.Round(total*100) / 100,
		Items: items,
	})
}

// StatsByMonth отдаёт в JSON суммы трат по месяцам года: /api/stats/by-month?year=2026
func (h *Handler) StatsByMonth(w http.ResponseWriter, r *http.Request) {
	year, err := readYear(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	months, err := h.repo.SumByMonth(currentUserID(r), year)
	if err != nil {
		log.Println("ошибка статистики по месяцам:", err)
		writeJSONError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	writeJSON(w, http.StatusOK, monthStatsResponse{Year: year, Months: months})
}

// readPeriod достаёт необязательные параметры from и to и проверяет их формат.
func readPeriod(r *http.Request) (from, to string, err error) {
	query := r.URL.Query()
	from = query.Get("from")
	to = query.Get("to")

	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			return "", "", errors.New("дата «с» должна быть в формате ГГГГ-ММ-ДД")
		}
	}
	if to != "" {
		if _, err := time.Parse("2006-01-02", to); err != nil {
			return "", "", errors.New("дата «по» должна быть в формате ГГГГ-ММ-ДД")
		}
	}
	if from != "" && to != "" && from > to {
		return "", "", errors.New("дата «с» не может быть позже даты «по»")
	}

	return from, to, nil
}

// readYear достаёт параметр year; если его нет - берёт текущий год.
func readYear(r *http.Request) (int, error) {
	s := r.URL.Query().Get("year")
	if s == "" {
		return time.Now().Year(), nil
	}

	year, err := strconv.Atoi(s)
	if err != nil || year < 2000 || year > 2100 {
		return 0, errors.New("год должен быть числом от 2000 до 2100")
	}
	return year, nil
}

// writeJSON отправляет data в виде JSON с заголовком Content-Type: application/json.
func writeJSON(w http.ResponseWriter, status int, data any) {
	// сначала превращаем в JSON, и только потом пишем ответ:
	// так при ошибке кодирования можно ещё честно ответить кодом 500
	body, err := json.Marshal(data)
	if err != nil {
		log.Println("ошибка кодирования JSON:", err)
		http.Error(w, "внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// writeJSONError отправляет ошибку в JSON: {"error": "текст"}.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
