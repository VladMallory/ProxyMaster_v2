// main.go
package main

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
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
	remnaClient := remnawave.NewRemnaClient(cfg)

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
