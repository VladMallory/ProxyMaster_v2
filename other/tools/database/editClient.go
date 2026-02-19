package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/database"
	"github.com/VladMallory/ProxyMaster_v2/internal/models"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type consoleLogger struct{}

func (c *consoleLogger) Debug(string, ...logger.Field) {}
func (c *consoleLogger) Info(string, ...logger.Field)  {}
func (c *consoleLogger) Warn(string, ...logger.Field)  {}

func (c *consoleLogger) Error(msg string, fields ...logger.Field) {
	for _, f := range fields {
		log.Fatalf("%s=%v", f.Key, f.Value)
	}

	log.Fatalln(msg)
}

func (c *consoleLogger) With(...logger.Field) logger.Logger {
	return c // игнорируем поля для простоты
}

func (c *consoleLogger) Named(string) logger.Logger {
	return c // игнорируем имя для простоты
}

func (c *consoleLogger) Sync() error {
	return nil
}

// AI: писала нейронка.
//
//nolint:cyclop, funlen
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

	for i := range maxRetries {
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

	// Интерактивное меню для удаления пользователя
	fmt.Println("\n🗑️ Удаление пользователя:")
	fmt.Println("Введите ID пользователя для удаления или 'q' для выхода:")
	fmt.Print("> ")

	var input string

	_, err = fmt.Scanln(&input)
	if err != nil {
		return
	}

	// Проверяем, хочет ли пользователь выйти
	if input == "q" || input == "Q" {
		fmt.Println("👋 Выход из программы")

		return
	}

	// Ищем пользователя в списке
	var foundUser *models.UserTG

	for i := range users {
		if users[i].ID == input {
			foundUser = &users[i]

			break
		}
	}

	// Если пользователь не найден - сообщаем и выходим
	if foundUser == nil {
		fmt.Printf("❌ Пользователь с ID '%s' не найден\n", input)

		return
	}

	// Подтверждение удаления - важная мера безопасности
	// Предотвращает случайное удаление
	fmt.Printf("\n⚠️ Вы уверены, что хотите удалить пользователя?\n")
	fmt.Printf("   ID: %s\n", foundUser.ID)
	fmt.Printf("   Пробный период: %v\n", foundUser.Trial)
	fmt.Printf("   Доп. устройств: %d\n", foundUser.ExtraDevicesCount)
	fmt.Println("\nВведите 'yes' для подтверждения удаления:")
	fmt.Print("> ")

	var confirm string

	_, err = fmt.Scanln(&confirm)
	if err != nil {
		return
	}

	if confirm != "yes" && confirm != "YES" && confirm != "Yes" {
		fmt.Println("❌ Удаление отменено")

		return
	}

	// Удаляем пользователя из базы
	fmt.Println("\n🔄 Удаляю пользователя из базы данных...")

	if err := deleteUserByID(db, input); err != nil {
		fmt.Printf("❌ Ошибка при удалении: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Пользователь успешно удален из базы данных")

	// Показываем обновленный список
	fmt.Println("\n📋 Обновленный список пользователей:")

	updatedUsers, err := userStorage.GetAllUsers()
	if err != nil {
		fmt.Printf("❌ Ошибка получения обновленного списка: %v\n", err)
		os.Exit(1)
	}

	printUsersTable(updatedUsers)
}

// deleteUserByID удаляет пользователя из базы данных по его ID
// Выполняет прямой SQL запрос DELETE для удаления записи.
func deleteUserByID(db *sqlx.DB, userID string) error {
	// Выполняем запрос на удаление
	// WHERE id = $1 - используем параметризованный запрос для безопасности (SQL injection protection)
	_, err := db.Exec("DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("ошибка удаления пользователя: %w", err)
	}

	return nil
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
			0,
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
	var trialUsers int

	var totalExtraDevices int

	for _, user := range users {
		totalExtraDevices += user.ExtraDevicesCount

		if user.Trial {
			trialUsers++
		}
	}

	fmt.Println("\n📊 Статистика:")
	fmt.Println("├─────────────────────────────────────────────")
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
		start := max(len(days)-7, 0)

		for i := start; i < len(days); i++ {
			fmt.Printf("│ %s: %d пользователей\n", days[i], registrationByDay[days[i]])
		}

		fmt.Println("└─────────────────────────────────────────────")
	}
}
