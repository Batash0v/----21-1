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
	db, err := repository.OpenDB("expenses.db", "migrations/001_create_expenses.sql")
	if err != nil {
		log.Fatal("не удалось открыть базу данных: ", err)
	}
	defer db.Close()

	repo := repository.NewExpenseRepository(db)

	h, err := handlers.New(repo)
	if err != nil {
		log.Fatal("не удалось загрузить шаблоны: ", err)
	}

	fmt.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", h.Routes()))
}
