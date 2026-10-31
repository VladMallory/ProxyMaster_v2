package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/VladMallory/ProxyMaster_v2/internal/payment/autopay"
)

func main() {
	username := flag.String("user", "", "Telegram ID пользователя")
	months := flag.Int("months", 1, "срок подписки при привязке")
	flag.Parse()

	cfg, err := autopay.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	if !cfg.Autopay {
		log.Println("YOOKASSA_AUTOPAY выключен")

		return
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	db, err := autopay.OpenDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Print(err)

		return
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			log.Printf("закрытие базы: %v", cerr)
		}
	}()

	if *username != "" {
		if err := autopay.Bind(ctx, cfg, db, *username, *months); err != nil {
			log.Print(err)
		}

		return
	}

	if err := autopay.RunWorker(ctx, cfg, db); err != nil {
		log.Print(err)
	}
}
