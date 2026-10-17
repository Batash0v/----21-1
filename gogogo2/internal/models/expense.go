// internal/models/expense.go
package models

// Expense - одна трата.
type Expense struct {
	ID          int
	Amount      float64 // сумма в рублях
	Description string
	Date        string // дата в формате 2006-01-02 (так её отдаёт <input type="date">)
}
