// internal/repository/expense_repository.go
package repository

import (
	"database/sql"
	"errors"

	"example.com/myserver/internal/models"
)

// ErrNotFound возвращается, когда траты с таким id нет в базе.
var ErrNotFound = errors.New("трата не найдена")

// ExpenseRepository содержит все SQL-запросы для таблицы expenses.
type ExpenseRepository struct {
	db *sql.DB
}

func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

// Create добавляет трату и возвращает её id.
func (r *ExpenseRepository) Create(e models.Expense) (int, error) {
	// значения передаются через ?, а не склеиваются в строку - это защита от SQL-инъекций
	res, err := r.db.Exec(
		"INSERT INTO expenses (amount, description, date) VALUES (?, ?, ?)",
		e.Amount, e.Description, e.Date,
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

// GetAll возвращает все траты: сначала новые даты.
func (r *ExpenseRepository) GetAll() ([]models.Expense, error) {
	rows, err := r.db.Query(
		"SELECT id, amount, description, date FROM expenses ORDER BY date DESC, id DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		if err := rows.Scan(&e.ID, &e.Amount, &e.Description, &e.Date); err != nil {
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

	err := r.db.QueryRow(
		"SELECT id, amount, description, date FROM expenses WHERE id = ?",
		id,
	).Scan(&e.ID, &e.Amount, &e.Description, &e.Date)

	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// Update обновляет трату с id = e.ID.
func (r *ExpenseRepository) Update(e models.Expense) error {
	res, err := r.db.Exec(
		"UPDATE expenses SET amount = ?, description = ?, date = ? WHERE id = ?",
		e.Amount, e.Description, e.Date, e.ID,
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
