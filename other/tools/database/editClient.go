package main

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/pkg/logger"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type consoleLogger struct{}

func (c *consoleLogger) Debug(msg string, fields ...logger.Field) {}
func (c *consoleLogger) Info(msg string, fields ...logger.Field)  {}
func (c *consoleLogger) Warn(msg string, fields ...logger.Field)  {}

func (c *consoleLogger) Error(msg string, fields ...logger.Field) {
	log.Fatalf("❌ [ERROR] %s", msg)

	for _, f := range fields {
		log.Fatalf("%s=%v", f.Key, f.Value)
	}
}

func (c *consoleLogger) With(fields ...logger.Field) logger.Logger {
	return c // игнорируем поля для простоты
}

func (c *consoleLogger) Named(name string) logger.Logger {
	return c // игнорируем имя для простоты
}

func (c *consoleLogger) Sync() error {
	return nil
}

// AI: писала нейронка.
func main() {
	// Настройка подключения к базе данных
	// Используем те же параметры, что и в основном приложении
	// Пароль берем из .env файла: userspass
	dbURL := "postgres://user:userspass@localhost:5432/usersdb?sslmode=disable"

	fmt.Println("🔗 Подключаюсь к базе данных...")
	fmt.Println("   URL:", dbURL)

	// Пытаемся подключиться с несколькими попытками
	var db *sqlx.DB

	var err error

	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		db, err = sqlx.Connect("postgres", dbURL)
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			fmt.Printf("   Попытка %d/%d не удалась: %v\n", i+1, maxRetries, err)
			fmt.Println("   Жду 2 секунды перед повторной попыткой...")
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		fmt.Printf("❌ Ошибка подключения к базе данных: %v\n", err)
		fmt.Println("\nВозможные причины:")
		fmt.Println("1. База данных не запущена")
		fmt.Println("   Запустите: make db-only")
		fmt.Println("2. Неправильные параметры подключения")
		fmt.Println("   Проверьте файл .env и docker-compose.dev.yml")
		fmt.Println("3. Порт 5432 занят другим приложением")
		os.Exit(1)
	}

	defer func(db *sqlx.DB) {
		err := db.Close()
		if err != nil {
			return
		}
	}(db)

	// Проверяем, что база отвечает
	if err := db.Ping(); err != nil {
		fmt.Printf("❌ База данных не отвечает: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Подключение к базе данных установлено")

	// Создаем логгер клиент
	loggerClient := &consoleLogger{}

	// Создаем хранилище пользователей
	userStorage := database.NewUserStorage(db, loggerClient)

	// Получаем всех пользователей
	fmt.Println("\n📋 Загружаю список пользователей...")

	users, err := userStorage.GetAllUsers()
	if err != nil {
		fmt.Printf("❌ Ошибка получения пользователей: %v\n", err)
		os.Exit(1)
	}

	// Выводим красиво в терминале
	printUsersTable(users)
}

// printUsersTable выводит таблицу пользователей в красивом формате.
func printUsersTable(users []models.UserTG) {
	if len(users) == 0 {
		fmt.Println("📭 В базе данных нет пользователей")
	}

	fmt.Printf("\n👥 Всего пользователей: %d\n\n", len(users))

	// Заголовок таблицы
	fmt.Println("┌────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ ID пользователя │ Баланс │ Пробный │ Доп. устройств │ Дата регистрации     │")
	fmt.Println("├─────────────────┼────────┼─────────┼────────────────┼──────────────────────┤")

	// Данные пользователей
	for _, user := range users {
		// Форматируем данные
		trialStatus := "Нет"
		if user.Trial {
			trialStatus = "Да"
		}

		// Форматируем дату
		regDate := user.CreatedAt.Format("02.01.2006 15:04")

		// Выводим строку
		fmt.Printf("│ %-15s │ %6d ₽ │ %-7s │ %14d │ %-20s │\n",
			user.ID,
			user.Balance,
			trialStatus,
			user.ExtraDevicesCount,
			regDate,
		)
	}

	// Нижняя граница таблицы
	fmt.Println("└─────────────────┴────────┴─────────┴────────────────┴──────────────────────┘")

	// Статистика
	printStatistics(users)
}

// printStatistics выводит статистику по пользователям.
func printStatistics(users []models.UserTG) {
	var totalBalance int

	var trialUsers int

	var totalExtraDevices int

	for _, user := range users {
		totalBalance += user.Balance
		totalExtraDevices += user.ExtraDevicesCount

		if user.Trial {
			trialUsers++
		}
	}

	avgBalance := 0
	if len(users) > 0 {
		avgBalance = totalBalance / len(users)
	}

	fmt.Println("\n📊 Статистика:")
	fmt.Println("├─────────────────────────────────────────────")
	fmt.Printf("│ Общий баланс всех пользователей: %d ₽\n", totalBalance)
	fmt.Printf("│ Средний баланс на пользователя: %d ₽\n", avgBalance)
	fmt.Printf("│ Пользователей на пробном периоде: %d/%d (%.1f%%)\n",
		trialUsers, len(users), float64(trialUsers)/float64(len(users))*100)
	fmt.Printf("│ Всего дополнительных устройств: %d\n", totalExtraDevices)
	fmt.Printf("│ Общий доход от доп. устройств: %d ₽/мес\n", totalExtraDevices*50)
	fmt.Println("└─────────────────────────────────────────────")

	// Группировка по датам регистрации
	printRegistrationStats(users)
}

// printRegistrationStats показывает статистику регистраций по дням.
func printRegistrationStats(users []models.UserTG) {
	// Группируем по дням
	registrationByDay := make(map[string]int)

	for _, user := range users {
		day := user.CreatedAt.Format("02.01.2006")
		registrationByDay[day]++
	}

	if len(registrationByDay) > 1 {
		fmt.Println("\n📅 Регистрации по дням:")
		fmt.Println("├─────────────────────────────────────────────")

		// Сортируем дни по дате
		var days []string
		for day := range registrationByDay {
			days = append(days, day)
		}

		// Простая сортировка (можно улучшить)
		for i := 0; i < len(days)-1; i++ {
			for j := i + 1; j < len(days); j++ {
				d1, _ := time.Parse("02.01.2006", days[i])
				d2, _ := time.Parse("02.01.2006", days[j])

				if d1.After(d2) {
					days[i], days[j] = days[j], days[i]
				}
			}
		}

		// Выводим последние 7 дней
		start := len(days) - 7
		if start < 0 {
			start = 0
		}

		for i := start; i < len(days); i++ {
			fmt.Printf("│ %s: %d пользователей\n", days[i], registrationByDay[days[i]])
		}

		fmt.Println("└─────────────────────────────────────────────")
	}
}
