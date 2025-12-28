package main

import (
	"ProxyMaster_v2/internal/app"
	"log"
)

func main() {
	// сборка приложения
	var myApp app.Application
	myApp, err := app.New()
	if err != nil {
		log.Fatal("ошибка сборки приложения", err)
	}

	// запуск приложения
	myApp.Run()
}
