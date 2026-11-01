// internal/models/expense.go
package models

// Expense - одна трата.
type Expense struct {
	ID           int
	Amount       float64 // сумма в рублях
	Description  string
	Date         string // дата в формате 2006-01-02 (так её отдаёт <input type="date">)
	CategoryID   int    // 0 - категория не задана
	CategoryName string // название категории (приходит из таблицы categories через JOIN)
}

// ExpenseFilter - условия фильтрации списка трат.
// Пустое значение поля означает "этот фильтр не задан".
type ExpenseFilter struct {
	CategoryID int    // 0 - все категории
	DateFrom   string // "" - без нижней границы
	DateTo     string // "" - без верхней границы
}
