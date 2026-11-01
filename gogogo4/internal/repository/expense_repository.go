// internal/repository/expense_repository.go
package repository

import (
	"database/sql"
	"errors"
	"strings"

	"example.com/myserver/internal/models"
)

// ErrNotFound возвращается, когда записи с таким id нет в базе.
var ErrNotFound = errors.New("запись не найдена")

// ExpenseRepository содержит все SQL-запросы для таблицы expenses.
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

// Create добавляет трату и возвращает её id.
func (r *ExpenseRepository) Create(e models.Expense) (int, error) {
	// значения передаются через ?, а не склеиваются в строку - это защита от SQL-инъекций
	res, err := r.db.Exec(
		"INSERT INTO expenses (amount, description, date, category_id) VALUES (?, ?, ?, ?)",
		e.Amount, e.Description, e.Date, e.CategoryID,
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

// GetAll возвращает траты с учётом фильтра: сначала новые даты.
func (r *ExpenseRepository) GetAll(f models.ExpenseFilter) ([]models.Expense, error) {
	query := selectExpense

	// Условия WHERE добавляем только для тех фильтров, которые заданы.
	// Сами значения всё равно идут через ?, в текст запроса попадают только фиксированные куски.
	var conditions []string
	var args []any

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

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
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

// GetByID возвращает одну трату или ErrNotFound.
func (r *ExpenseRepository) GetByID(id int) (models.Expense, error) {
	var e models.Expense

	err := r.db.QueryRow(selectExpense+" WHERE e.id = ?", id).
		Scan(&e.ID, &e.Amount, &e.Description, &e.Date, &e.CategoryID, &e.CategoryName)

	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// Update обновляет трату с id = e.ID.
func (r *ExpenseRepository) Update(e models.Expense) error {
	res, err := r.db.Exec(
		"UPDATE expenses SET amount = ?, description = ?, date = ?, category_id = ? WHERE id = ?",
		e.Amount, e.Description, e.Date, e.CategoryID, e.ID,
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

// Delete удаляет трату по id.
func (r *ExpenseRepository) Delete(id int) error {
	res, err := r.db.Exec("DELETE FROM expenses WHERE id = ?", id)
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
