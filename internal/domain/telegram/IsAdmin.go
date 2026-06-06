package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/database"
	"go.uber.org/zap"
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

	l *zap.Logger,
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
				zap.Int64("ChatID", chatID),
				zap.Error(err),
			)

			return false, fmt.Errorf("ошибка отправки сообщения о нехватке прав доступа: %w", err)
		}

		l.Info("У пользователя нет прав доступа к админской команде",
			zap.Int64("ChatID", chatID),
			zap.String("Command", command))

		return true, nil
	}

	// Если админ, обрабатываем команду
	// Извлекаем текст сообщения для рассылки.
	// TrimSpace используется для удаления лишних пробелов.
	message := strings.TrimSpace(strings.TrimPrefix(command, Distribution))
	if message == "" {
		l.Warn("Админ отправил команду /distribution без текста сообщения",
			zap.Int64("ChatID", chatID))

		if err := sender.SendMessage(
			chatID,
			"Пожалуйста, введите сообщение для рассылки.",
		); err != nil {
			l.Error("Ошибка отправки сообщения с просьбой ввести текст",
				zap.Int64("ChatID", chatID),
				zap.Error(err))

			return false, fmt.Errorf("ошибка отправки сообщения с просьбой ввести текст: %w", err)
		}

		return true, nil
	}

	// Получаем ID всех активных пользователей из базы данных.
	userIDs, err := userRepo.GetActiveUserIDs()
	if err != nil {
		l.Error("Ошибка получения ID пользователей для рассылки",
			zap.Int64("ChatID", chatID),
			zap.Error(err))

		if sendErr := sender.SendMessage(
			chatID,
			"Не удалось получить список пользователей для рассылки.",
		); sendErr != nil {
			l.Error("Ошибка отправки сообщения об ошибке получения пользователей",
				zap.Int64("ChatID", chatID),
				zap.Error(sendErr))

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
		zap.Int64("ChatID", chatID),
		zap.Int("TotalUsers", len(userIDs)))

	go func() {
		successCount := 0

		for _, userIDStr := range userIDs {
			userID, err := strconv.ParseInt(userIDStr, 10, 64)
			if err != nil {
				l.Error("Ошибка конвертации ID пользователя",
					zap.String("UserIDStr", userIDStr),
					zap.Error(err))

				continue
			}
			// Отправляем сообщение каждому пользователю.
			if err := sender.SendMessage(userID, message); err != nil {
				l.Error("Ошибка отправки сообщения пользователю при рассылке",
					zap.Int64("UserID", userID),
					zap.Error(err))
			} else {
				successCount++
			}
			// Добавляем небольшую задержку между отправками, чтобы не превысить
			// лимиты Telegram API и избежать блокировки бота.
			time.Sleep(100 * time.Millisecond)
		}

		l.Info("Рассылка завершена",
			zap.Int64("ChatID", chatID),
			zap.Int("SuccessCount", successCount),
			zap.Int("TotalCount", len(userIDs)))
	}()

	// Сообщаем администратору, что рассылка успешно запущена.
	l.Info("Администратор запустил рассылку",
		zap.Int64("ChatID", chatID),
		zap.Int("UserCount", len(userIDs)))

	if err := sender.SendMessage(
		chatID,
		fmt.Sprintf("✅ Рассылка запущена для %d пользователей.", len(userIDs)),
	); err != nil {
		l.Error("Ошибка отправки подтверждения о запуске рассылки",
				zap.Int64("ChatID", chatID),
				zap.Error(err))

		return false, fmt.Errorf("ошибка отправки подтверждения о запуске рассылки: %w", err)
	}

	return true, nil
}
