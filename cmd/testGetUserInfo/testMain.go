// main.go
package main

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

type UserInfoRequest struct {
}

type GetUserInfoResponse struct {
}

func main() {
	//
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.New()
	remnaClient := remnawave.NewRemnaClient(cfg)

	user, err := remnaClient.GetUUIDByUsername("admin")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(user)
}
