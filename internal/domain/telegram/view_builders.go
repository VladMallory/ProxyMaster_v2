package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
)

// buildMainViewText строит текст главного меню с информацией о подписке пользователя
func buildMainViewText(
	chatID int64,
	firstName string,
	remnawaveClient domain.RemnawaveClient,
	userRepo *database.UserStorage,
) string {
	username := strconv.FormatInt(chatID, 10)

	_, err := userRepo.GetUserByID(username)
	if err != nil {
		// Если пользователя нету, создаем его в базе
		if errors.Is(err, domain.ErrUserNotFound) {
			_, createErr := userRepo.CreateUser(models.CreateUserTGDTO{
				ID:      username,
				Balance: 0,
				Trial:   false,
			})
			if createErr == nil {
				return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
			}
			// В случае ошибки создания, показываем с нулевым балансом
			log.Printf(
				"Не удалось создать пользователя %s при сборке текста: %v",
				username,
				createErr,
			)
			return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
		}
		// В случае другой ошибки, показываем с нулевым балансом
		log.Printf("Не удалось получить пользователя %s при сборке текста: %v", username, err)
		return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
	}

	return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
}

// buildSubscriptionLine строит строку со статусом подписки пользователя
// Возвращает строку вида "—✅ Подписка активна до 20 января" или "—❌ Подписка не активна"
func buildSubscriptionLine(username string, remnawaveClient domain.RemnawaveClient) string {
	uuid, err := remnawaveClient.GetUUIDByUsername(username)
	if err != nil {
		return "—❌ Подписка не активна"
	}

	info, err := remnawaveClient.GetUserInfo(uuid)
	if err != nil {
		return "—❌ Подписка не активна"
	}

	if strings.EqualFold(info.Response.Status, "ACTIVE") &&
		info.Response.ExpireAt.After(time.Now()) {
		return "—✅ Подписка активна до " + formatRussianDate(info.Response.ExpireAt)
	}

	return "—❌ Подписка не активна"
}

// buildProfileData строит данные профиля для отображения в меню
func buildProfileData(userID string, userRepo *database.UserStorage) (string, error) {
	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		return "", err
	}

	// Получаем 100% точный результат количества устройств
	extraCount, err := userRepo.CountActiveDeviceAddons(userID)
	if err != nil {
		return "", err
	}

	// Проверяем, совпадает ли счетчик с реальным количеством
	if user.ExtraDevicesCount != extraCount {
		_, err = userRepo.UpdateUser(userID, models.UpdateUserTGDTO{
			ExtraDevicesCount: &extraCount,
		})
		if err != nil {
			return "", err
		}
	}

	nextChargeAt, err := userRepo.GetNextDeviceAddonChargeAt(userID)
	if err != nil {
		return "", err
	}

	nextPayment := "—"
	if nextChargeAt != nil {
		nextPayment = formatDevicePaymentDate(*nextChargeAt, time.Now())
	}

	return user.ID + "|" + strconv.Itoa(user.Balance) + "|" + strconv.Itoa(extraCount) + "|" + nextPayment, nil
}
