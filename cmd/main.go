package main

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/pkg/logger"
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.New()

	loggerClient, err := logger.NewZap(cfg.LoggerLevel)
	if err != nil {
		log.Fatal("ошибка инициализации логгера: %w", err)
	}

	remnawaveLogger := loggerClient.Named("remnawave")

	remnaClient := remnawave.NewRemnaClient(cfg, remnawaveLogger)

	//получене uuid юзера
	uuid, err := remnaClient.GetUUIDByUsername("admin")
	if err != nil {
		log.Fatal(err)
	}

	//получение инфы о юзере//
	_, err = remnaClient.GetUserInfo(uuid)
	//получение статуса юзера
	status, err := remnaClient.GetUserStatus(uuid)

	fmt.Println(status)
}
