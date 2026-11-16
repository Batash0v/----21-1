// internal/repository/expense_repository.go
package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"example.com/myserver/internal/models"
)

// ErrNotFound возвращается, когда записи с таким id нет в базе.
var ErrNotFound = errors.New("запись не найдена")

// ExpenseRepository содержит все SQL-запросы для таблицы expenses.
// Каждый метод принимает userID и работает только с тратами этого пользователя.
type ExpenseRepository struct {
	db *sql.DB
}

func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

// selectExpense - общая часть SELECT. Название категории берём через JOIN,
// поэтому не нужен отдельный запрос на каждую трату (проблема N+1).
// LEFT JOIN, чтобы траты без категории тоже попадали в список.
const selectExpense = `
	SELECT e.id, e.amount, e.description, e.date,
	       COALESCE(e.category_id, 0), COALESCE(c.name, 'Без категории')
	FROM expenses e
	LEFT JOIN categories c ON c.id = e.category_id`

// Create добавляет трату пользователя userID и возвращает её id.
func (r *ExpenseRepository) Create(userID int, e models.Expense) (int, error) {
	// значения передаются через ?, а не склеиваются в строку - это защита от SQL-инъекций
	res, err := r.db.Exec(
		"INSERT INTO expenses (amount, description, date, category_id, user_id) VALUES (?, ?, ?, ?, ?)",
		e.Amount, e.Description, e.Date, e.CategoryID, userID,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// GetAll возвращает траты пользователя с учётом фильтра: сначала новые даты.
func (r *ExpenseRepository) GetAll(userID int, f models.ExpenseFilter) ([]models.Expense, error) {
	query := selectExpense

	// Условие по пользователю есть всегда, остальные - только если фильтр задан.
	// Сами значения идут через ?, в текст запроса попадают только фиксированные куски.
	conditions := []string{"e.user_id = ?"}
	args := []any{userID}

	if f.CategoryID > 0 {
		conditions = append(conditions, "e.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.DateFrom != "" {
		conditions = append(conditions, "e.date >= ?")
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		conditions = append(conditions, "e.date <= ?")
		args = append(args, f.DateTo)
	}

	query += " WHERE " + strings.Join(conditions, " AND ")
	query += " ORDER BY e.date DESC, e.id DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		err := rows.Scan(&e.ID, &e.Amount, &e.Description, &e.Date, &e.CategoryID, &e.CategoryName)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return expenses, nil
}

// GetByID возвращает трату пользователя или ErrNotFound.
// Чужая трата тоже даёт ErrNotFound: пользователь не должен даже узнать, что она существует.
func (r *ExpenseRepository) GetByID(id, userID int) (models.Expense, error) {
	var e models.Expense

	err := r.db.QueryRow(selectExpense+" WHERE e.id = ? AND e.user_id = ?", id, userID).
		Scan(&e.ID, &e.Amount, &e.Description, &e.Date, &e.CategoryID, &e.CategoryName)

	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// Update обновляет трату с id = e.ID, если она принадлежит пользователю userID.
func (r *ExpenseRepository) Update(userID int, e models.Expense) error {
	res, err := r.db.Exec(
		"UPDATE expenses SET amount = ?, description = ?, date = ?, category_id = ? WHERE id = ? AND user_id = ?",
		e.Amount, e.Description, e.Date, e.CategoryID, e.ID, userID,
	)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete удаляет трату по id, если она принадлежит пользователю userID.
func (r *ExpenseRepository) Delete(id, userID int) error {
	res, err := r.db.Exec("DELETE FROM expenses WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SumByCategory возвращает сумму трат пользователя по каждой категории за период.
// from и to необязательны (пустая строка - граница не задана), даты включаются в период.
// Суммирует и группирует сама база данных: в Go приходит уже готовый короткий список.
func (r *ExpenseRepository) SumByCategory(userID int, from, to string) ([]models.CategoryStat, error) {
	query := `
		SELECT COALESCE(c.name, 'Без категории'), ROUND(SUM(e.amount), 2)
		FROM expenses e
		LEFT JOIN categories c ON c.id = e.category_id
		WHERE e.user_id = ?`
	args := []any{userID}

	if from != "" {
		query += " AND e.date >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND e.date <= ?"
		args = append(args, to)
	}

	query += `
		GROUP BY COALESCE(c.name, 'Без категории')
		ORDER BY SUM(e.amount) DESC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// make, а не var: пустой результат должен превратиться в JSON [], а не в null
	stats := make([]models.CategoryStat, 0)
	for rows.Next() {
		var s models.CategoryStat
		if err := rows.Scan(&s.Name, &s.Total); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

// SumByMonth возвращает сумму трат пользователя по месяцам выбранного года.
// Всегда возвращается ровно 12 элементов: у месяцев без трат сумма равна 0.
func (r *ExpenseRepository) SumByMonth(userID int, year int) ([]models.MonthStat, error) {
	rows, err := r.db.Query(`
		SELECT CAST(strftime('%m', date) AS INTEGER), ROUND(SUM(amount), 2)
		FROM expenses
		WHERE user_id = ? AND date >= ? AND date <= ?
		GROUP BY strftime('%m', date)`,
		userID, fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	months := make([]models.MonthStat, 12)
	for i := range months {
		months[i].Month = i + 1
	}

	for rows.Next() {
		var month int
		var total float64
		if err := rows.Scan(&month, &total); err != nil {
			return nil, err
		}
		if month >= 1 && month <= 12 {
			months[month-1].Total = total
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return months, nil
}
