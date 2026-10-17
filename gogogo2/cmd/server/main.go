// cmd/server/main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"example.com/myserver/internal/handlers"
)

func main() {
	h, err := handlers.New()
	if err != nil {
		log.Fatal("не удалось загрузить шаблоны: ", err)
	}

	fmt.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", h.Routes()))
}
