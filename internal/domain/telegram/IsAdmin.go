package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/database"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

// Distribution Админские команды.
const (
	Distribution = "/distribution"
	Check        = "/check"
)

// isAdmin проверяет, является ли пользователь администратором и обрабатывает админские команды.
func isAdmin(sender MessageSender,
	chatID int64,
	command string,
	// firstName string,
	// remnawaveClient domain.RemnawaveClient,
	adminID int64,
	userRepo *database.UserStorage,

	l logger.Logger,
) (bool, error) {
	command = strings.TrimSpace(command)

	// Проверка на команду /check — не админская, просто проверяет права.
	if strings.HasPrefix(command, Check) {
		if chatID == adminID {
			_ = sender.SendMessage(chatID, "Вы админ ✅")
		} else {
			_ = sender.SendMessage(chatID, "Вы не админ ❌")
		}

		return true, nil
	}

	// проверка на админскую команду
	// если команда не админская, выходим сразу без логгирования
	if !strings.HasPrefix(command, Distribution) {
		return false, nil
	}

	// если команда админская, проверяем права пользователя
	// Выходим если не админ и логгируем попытку доступа
	if chatID != adminID {
		if err := sender.SendMessage(chatID, "У вас нет прав для этой команды."); err != nil {
			l.Error("Ошибка отправки сообщения о нехватке прав доступа",
				logger.Field{Key: "ChatID", Value: chatID},
				logger.Field{Key: "error", Value: err},
			)

			return false, fmt.Errorf("ошибка отправки сообщения о нехватке прав доступа: %w", err)
		}

		l.Info("У пользователя нет прав доступа к админской команде",
			logger.Field{Key: "ChatID", Value: chatID},
			logger.Field{Key: "Command", Value: command})

		return true, nil
	}

	// Если админ, обрабатываем команду
	// Извлекаем текст сообщения для рассылки.
	// TrimSpace используется для удаления лишних пробелов.
	message := strings.TrimSpace(strings.TrimPrefix(command, Distribution))
	if message == "" {
		l.Warn("Админ отправил команду /distribution без текста сообщения",
			logger.Field{Key: "ChatID", Value: chatID})

		if err := sender.SendMessage(
			chatID,
			"Пожалуйста, введите сообщение для рассылки.",
		); err != nil {
			l.Error("Ошибка отправки сообщения с просьбой ввести текст",
				logger.Field{Key: "ChatID", Value: chatID},
				logger.Field{Key: "error", Value: err})

			return false, fmt.Errorf("ошибка отправки сообщения с просьбой ввести текст: %w", err)
		}

		return true, nil
	}

	// Получаем ID всех активных пользователей из базы данных.
	userIDs, err := userRepo.GetActiveUserIDs()
	if err != nil {
		l.Error("Ошибка получения ID пользователей для рассылки",
			logger.Field{Key: "ChatID", Value: chatID},
			logger.Field{Key: "error", Value: err})

		if sendErr := sender.SendMessage(
			chatID,
			"Не удалось получить список пользователей для рассылки.",
		); sendErr != nil {
			l.Error("Ошибка отправки сообщения об ошибке получения пользователей",
				logger.Field{Key: "ChatID", Value: chatID},
				logger.Field{Key: "error", Value: sendErr})

			return false, fmt.Errorf(
				"ошибка отправки сообщения о неудаче получения списка пользователей: %w",
				sendErr,
			)
		}

		return true, nil
	}

	// Запускаем рассылку в отдельной горутине, чтобы не блокировать основной поток.
	// Это важно для асинхронной обработки и позволяет боту оставаться отзывчивым.
	l.Info("Начало рассылки сообщения",
		logger.Field{Key: "ChatID", Value: chatID},
		logger.Field{Key: "TotalUsers", Value: len(userIDs)})

	go func() {
		successCount := 0

		for _, userIDStr := range userIDs {
			userID, err := strconv.ParseInt(userIDStr, 10, 64)
			if err != nil {
				l.Error("Ошибка конвертации ID пользователя",
					logger.Field{Key: "UserIDStr", Value: userIDStr},
					logger.Field{Key: "error", Value: err})

				continue
			}
			// Отправляем сообщение каждому пользователю.
			if err := sender.SendMessage(userID, message); err != nil {
				l.Error("Ошибка отправки сообщения пользователю при рассылке",
					logger.Field{Key: "UserID", Value: userID},
					logger.Field{Key: "error", Value: err})
			} else {
				successCount++
			}
			// Добавляем небольшую задержку между отправками, чтобы не превысить
			// лимиты Telegram API и избежать блокировки бота.
			time.Sleep(100 * time.Millisecond)
		}

		l.Info("Рассылка завершена",
			logger.Field{Key: "ChatID", Value: chatID},
			logger.Field{Key: "SuccessCount", Value: successCount},
			logger.Field{Key: "TotalCount", Value: len(userIDs)})
	}()

	// Сообщаем администратору, что рассылка успешно запущена.
	l.Info("Администратор запустил рассылку",
		logger.Field{Key: "ChatID", Value: chatID},
		logger.Field{Key: "UserCount", Value: len(userIDs)})

	if err := sender.SendMessage(
		chatID,
		fmt.Sprintf("✅ Рассылка запущена для %d пользователей.", len(userIDs)),
	); err != nil {
		l.Error("Ошибка отправки подтверждения о запуске рассылки",
			logger.Field{Key: "ChatID", Value: chatID},
			logger.Field{Key: "error", Value: err})

		return false, fmt.Errorf("ошибка отправки подтверждения о запуске рассылки: %w", err)
	}

	return true, nil
}
