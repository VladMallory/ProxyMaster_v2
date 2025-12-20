package main

import (
	"ProxyMaster_v2/internal/app"
	"log"
)

func main() {
	// 1. сборка приложения
	myApp, err := app.New()
	if err != nil {
		log.Fatal("ошибка сборки приложения: %w", err)
	}

	// 1. запуск приложения
	myApp.Run()
}
