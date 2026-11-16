// internal/models/stats.go
package models

// CategoryStat - сумма трат по одной категории.
// Теги json нужны, чтобы структура превращалась в JSON с короткими именами полей.
type CategoryStat struct {
	Name  string  `json:"name"`
	Total float64 `json:"total"`
}

// MonthStat - сумма трат за один месяц (Month от 1 до 12).
type MonthStat struct {
	Month int     `json:"month"`
	Total float64 `json:"total"`
}
