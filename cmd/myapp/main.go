// package main
package main

import (
	"log"

	"ProxyMaster_v2/internal/app"
)

func main() {
	// сборка приложения
	var myApp app.ProgramDADA
	myApp, err := app.New()
	if err != nil {
		log.Fatal("ошибка сборки приложения", err)
	}

	app.ProgramDADA.RunAPP()

	// запуск приложения
	myApp.RunAPP()
}
