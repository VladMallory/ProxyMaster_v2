package main

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/infrastructure/telegram"
	"log"
	"os"
)

func main() {

	config.New()

	//создаем бота
	bot, err := telegram.NewBot(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	//запускаем бота
	err = bot.Run()
	if err != nil {
		log.Fatal(err)
	}

	// todo: Либо можно сделать так: пакет где лежит бот переименовать в telegramBot
	// todo: и как нам нужен будет бот писать
	// todo: telegramBot.Run(), и всё что связано с ботом будет скрыто
	//
	//
	// todo: Что бы так было
	// package main
	//
	// func main(){
	//	bot := telegramBot.Run(os.Getenv("TELEGRAM_TOKEN"))
	//	}
}
