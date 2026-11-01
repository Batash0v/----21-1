// internal/repository/category_repository.go
package repository

import (
	"database/sql"
	"errors"

	"example.com/myserver/internal/models"
)

// CategoryRepository - SQL-запросы для таблицы categories.
type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetAll возвращает все категории.
func (r *CategoryRepository) GetAll() ([]models.Category, error) {
	rows, err := r.db.Query("SELECT id, name FROM categories ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// GetByID возвращает категорию или ErrNotFound.
func (r *CategoryRepository) GetByID(id int) (models.Category, error) {
	var c models.Category

	err := r.db.QueryRow("SELECT id, name FROM categories WHERE id = ?", id).Scan(&c.ID, &c.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}
