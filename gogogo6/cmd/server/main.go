// cmd/server/main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"example.com/myserver/internal/handlers"
	"example.com/myserver/internal/repository"
)

func main() {
	// _pragma=foreign_keys(1) включает проверку внешних ключей (в SQLite она по умолчанию выключена)
	db, err := repository.OpenDB("expenses.db?_pragma=foreign_keys(1)", "migrations")
	if err != nil {
		log.Fatal("не удалось открыть базу данных: ", err)
	}
	defer db.Close()

	expenseRepo := repository.NewExpenseRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	h, err := handlers.New(expenseRepo, categoryRepo, userRepo, sessionRepo)
	if err != nil {
		log.Fatal("не удалось загрузить шаблоны: ", err)
	}

	fmt.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", h.Routes()))
}
