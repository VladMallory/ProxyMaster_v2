//nolint:paralleltest, lll, golines, typecheck
package remnawave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

// TestGetUUIDByUsername самый простой тест для получения UUID.
// helperSetupClient создает тестовый сервер и клиент для тестов
// Возвращает клиент и сервер. Вызывающий должен вызвать server.Close().
func helperSetupClient(responseHandler http.HandlerFunc) (*RemnaClient, *httptest.Server) { //nolint:golines
	server := httptest.NewServer(responseHandler)

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	return client, server
}

// helperSetupClientWithMethod создает тестовый сервер и клиент с проверкой HTTP метода
// шаблонная функция для тестов с проверкой метода
// использует тот же принцип DRY (Don't Repeat Yourself)
// соответствует принципу SRP (Single Responsibility Principle)
// возвращает клиент и сервер для дальнейшей работы
// параметр expectedMethod указывает ожидаемый HTTP метод
// параметр urlPath указывает путь для проверки запроса.
func helperSetupClientWithMethod(expectedMethod string, urlPath string) (*RemnaClient, *httptest.Server) { //nolint:golines
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// helperSetupClientWithMethod проверяем соответствие метода
		if r.URL.Path == urlPath {
			if r.Method != expectedMethod {
				http.Error(w, fmt.Sprintf("Ожидали метод %s, получили %s", expectedMethod, r.Method), http.StatusBadRequest)

				return
			}

			w.WriteHeader(http.StatusOK)
		}
	}))

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	return client, server
}

func TestGetUUIDByUsername(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем правильность запроса
		if r.URL.Path != "/api/users/by-username/testUser" {
			t.Errorf("Неправильный путь: %s", r.URL.Path)
		}

		if !strings.Contains(r.URL.RawQuery, "testToken") {
			t.Errorf("Отсутствует токен в запросе")
		}

		if r.Header.Get("Authorization") != "Bearer testKey" {
			t.Errorf("Неверный Authorization заголовок")
		}

		// Отправляем успешный ответ
		response := map[string]interface{}{
			"response": map[string]string{
				"uuid":     "11111111-2222-3333-4444-555555555555",
				"username": "testUser",
			},
		}

		w.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}))
	defer server.Close()

	// Настраиваем клиент
	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	// Запускаем тест
	gotUUID, err := client.GetUUIDByUsername(context.Background(), "testUser")
	// Проверяем результат
	if err != nil {
		t.Errorf("Ожидали успех, получили ошибку: %v", err)
	}

	if gotUUID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("Неверный UUID: %s", gotUUID)
	}
}

// TestDeleteUser_success простой тест на успешное удаление.
func TestDeleteUser_success(t *testing.T) {
	fakeUUID := "11111111-2222-3333-4444-555555555555"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получение UUID по username
		if r.URL.Path == "/api/users/by-username/testUser" && r.URL.RawQuery == "testToken" {
			response := map[string]interface{}{
				"response": map[string]interface{}{
					"uuid":      fakeUUID,
					"id":        1,
					"shortUuid": "test-short-uuid",
					"username":  "testUser",
					"status":    "ACTIVE",
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}

			return
		}

		// Удаление пользователя по UUID
		if r.URL.Path == fmt.Sprintf("/api/users/%s", fakeUUID) && r.URL.RawQuery == "testToken" {
			if r.Method != "DELETE" {
				t.Errorf("Ожидали DELETE метод, получили: %s", r.Method)
			}

			w.WriteHeader(http.StatusNoContent)

			return
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	err := client.DeleteUser(context.Background(), "testUser")
	if err != nil {
		t.Errorf("Удаление должно пройти успешно, ошибка: %v", err)
	}
}

// TestDeleteUser_notFound тест на ошибку 404.
func TestDeleteUser_notFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Всегда возвращаем 404
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	err := client.DeleteUser(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Ожидали ошибку для несуществующего пользователя")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Ожидали ErrNotFound, получили: %v", err)
	}
}

// TestCreateUser_success тест на успешное создание пользователя.
func TestCreateUser_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users/" {
			// Проверяем что это POST запрос
			if r.Method != "POST" {
				t.Errorf("Ожидали POST метод, получили: %s", r.Method)
			}

			// Отправляем успешный ответ
			w.WriteHeader(http.StatusCreated)

			return
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
		RemnaSquadUUID:      "test-squad-uuid",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	dto := CreateUserDTO{
		Username: "new user",
		Days:     30,
	}
	err := client.CreateUser(dto)
	if err != nil {
		t.Errorf("Создание должно пройти успешно, ошибка: %v", err)
	}
}

// TestCreateUser_duplicate тест на повторное создание пользователя.
func TestCreateUser_duplicate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users/" {
			// Отправляем ошибку что пользователь уже существует
			w.WriteHeader(http.StatusBadRequest)

			_, err := w.Write([]byte(`User username already exists`))
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
		RemnaSquadUUID:      "test-squad-uuid",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	dto := CreateUserDTO{
		Username: "existing user",
		Days:     30,
	}
	err := client.CreateUser(dto)
	if err != nil {
		t.Errorf("Повторное создание должно пройти успешно, ошибка: %v", err)
	}
}

// TestSetTraffic_success тест на успешную установку трафика.
func TestSetTraffic_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users/" {
			// Проверяем метод PATCH
			if r.Method != "PATCH" {
				t.Errorf("Ожидали PATCH метод, получили: %s", r.Method)
			}

			// Отправляем успешный ответ
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	err := client.SetTraffic("testUser", 50)
	if err != nil {
		t.Errorf("Установка трафика должна пройти успешно, ошибка: %v", err)
	}
}

// TestGetUserInfo_success тест на успешное получение информации о пользователе.
func TestGetUserInfo_success(t *testing.T) {
	client, server := helperSetupClient(func(w http.ResponseWriter, r *http.Request) { //nolint:golines
		if r.URL.Path == "/api/users/test-uuid" {
			response := map[string]any{
				"response": map[string]string{
					"status": "ACTIVE",
				},
			}

			w.Header().Set("Content-Type", "application/json")

			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
		}
	})

	gotInfo, err := client.GetUserInfo("test-uuid")
	if err != nil {
		t.Errorf("Получение информации должно пройти успешно, ошибка: %v", err)
	}

	if gotInfo.Response.Status != "ACTIVE" {
		t.Errorf("Ожидали статус ACTIVE, получили: %s", gotInfo.Response.Status)
	}

	server.Close()
}

// TestEncryptURL_success тест на успешное шифрование URL.
func TestEncryptURL_success(t *testing.T) {
	fakeResponse := map[string]string{
		"encrypted_link": "encrypted-test-url",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api.php" {
			// Проверяем правильность запроса
			if !strings.Contains(r.URL.RawQuery, "key") {
				t.Errorf("Отсутствует ключ в запросе")
			}

			// Отправляем успешный ответ
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			err := json.NewEncoder(w).Encode(fakeResponse)
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	gotEncrypted, err := client.EncryptURL("test-url")
	if err != nil {
		t.Errorf("Шифрование должно пройти успешно, ошибка: %v", err)
	}
	// Проверяем что получили какой-то зашифрованный URL
	if gotEncrypted == "" {
		t.Error("Ожидали зашифрованный URL")
	}
}

// TestSetDevices_success тест на успешную установку количества устройств.
func TestSetDevices_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users/" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	devices := uint8(5)

	err := client.SetDevices(context.Background(), "testUser", &devices)
	if err != nil {
		t.Errorf("Установка устройств должна пройти успешно, ошибка: %v", err)
	}
}

// TestBetterResetTraffic_success тест на успешный сброс трафика.
func TestBetterResetTraffic_success(t *testing.T) {
	resetCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получение UUID
		if r.URL.Path == "/api/users/by-username/testUser" {
			response := map[string]interface{}{
				"response": map[string]string{
					"uuid":     "11111111-2222-3333-4444-555555555555",
					"username": "testUser",
				},
			}

			w.Header().Set("Content-Type", "application/json")

			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}

			return
		}

		// Сброс трафика
		if r.URL.Path == "/api/users/11111111-2222-3333-4444-555555555555/actions/reset-traffic" {
			if r.Method != "POST" {
				t.Errorf("Ожидали POST метод, получили: %s", r.Method)
			}

			resetCalled = true

			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	err := client.BetterResetTraffic(context.Background(), "testUser")
	if err != nil {
		t.Errorf("Сброс трафика должен пройти успешно, ошибка: %v", err)
	}

	if !resetCalled {
		t.Error("Сброс трафика не был вызван")
	}
}

// TestGetUserStatus_success тест на успешное получение статуса.
func TestGetUserStatus_success(t *testing.T) {
	client, server := helperSetupClient(func(w http.ResponseWriter, r *http.Request) { //nolint:golines
		if r.URL.Path == "/api/users/test-uuid" {
			response := map[string]interface{}{
				"response": map[string]string{
					"status": "ACTIVE",
				},
			}

			w.Header().Set("Content-Type", "application/json")

			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
		}
	})

	gotStatus, err := client.GetUserStatus("test-uuid")
	if err != nil {
		t.Errorf("Получение статуса должно пройти успешно, ошибка: %v", err)
	}

	if gotStatus != "ACTIVE" {
		t.Errorf("Ожидали статус ACTIVE, получили: %s", gotStatus)
	}

	server.Close()
}

// TestAddInternalSquad_success тест на успешное добавление внутренней команды.
// использует helperSetupClientWithMethod для проверки PATCH метода
// этот тест проверяет корректность выполнения API вызова AddInternalSquad.
func TestAddInternalSquad_success(t *testing.T) {
	client, server := helperSetupClientWithMethod("PATCH", "/api/users/testUser/actions/add-squad")

	squadTitles := []string{"squad1", "squad2"}

	err := client.AddInternalSquad(context.Background(), "testUser", squadTitles)
	if err != nil {
		t.Errorf("Добавление команды должно пройти успешно, ошибка: %v", err)
	}

	server.Close()
}

// TestAddInternalSquad_error тест на ошибку при добавлении команды.
func TestAddInternalSquad_error(t *testing.T) {
	errorResponse := map[string]interface{}{
		"error": "squad not found",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users/" {
			// Проверяем правильность запроса
			if r.Method != "PATCH" {
				t.Errorf("Ожидали PATCH метод, получили: %s", r.Method)
			}

			// Отправляем ошибку
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			err := json.NewEncoder(w).Encode(errorResponse)
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RemnaPanelURL:       server.URL,
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	squadTitles := []string{"squad1"}
	err := client.AddInternalSquad(context.Background(), "testUser", squadTitles)

	if err == nil {
		t.Error("Ожидали ошибку при добавлении команды")
	}

	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("Ожидали ErrBadRequest, получили: %v", err)
	}
}

// TestCreateUser_invalidDays тест на ошибку с некорректным количеством дней.
func TestCreateUser_invalidDays(t *testing.T) {
	cfg := &config.Config{
		RemnaPanelURL:       "http://example.com",
		RemnaSecretURLToken: "testToken",
		RemnaKey:            "testKey",
		RemnaSquadUUID:      "test-squad-uuid",
	}
	logClient, _ := logger.NewZap("debug")
	client := NewRemnaClient(cfg, logClient)

	// Должна быть ошибка так как days <= 0
	dto := CreateUserDTO{
		Username: "testUser",
		Days:     -1,
	}
	err := client.CreateUser(dto)
	if err == nil {
		t.Error("Ожидали ошибку для некорректного количества дней")
	}

	if !errors.Is(err, ErrDaysNotNill) {
		t.Errorf("Ожидали ErrDaysNotNil, получили: %v", err)
	}
}

// TestExtendClientSubscription_success тест на успешное продление подписки.
func TestExtendClientSubscription_success(t *testing.T) {
	client, server := helperSetupClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Ожидали POST метод, получили: %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
	})

	err := client.ExtendClientSubscription("test-uuid", "testUser", 30)
	if err != nil {
		t.Errorf("Продление должно пройти успешно, ошибка: %v", err)
	}

	server.Close()
}

// TestEnableClient_success тест на успешное включение клиента.
// использует helperSetupClientWithMethod для проверки PUT метода
// этот тест проверяет корректность выполнения API вызова EnableClient.
func TestEnableClient_success(t *testing.T) {
	client, server := helperSetupClientWithMethod("PUT", "/api/users/test-uuid/actions/enable")

	err := client.EnableClient("test-uuid")
	if err != nil {
		t.Errorf("Включение должно пройти успешно, ошибка: %v", err)
	}

	server.Close()
}

// TestDeleteDeviceHWID_success тест на успешное удаление устройств.
func TestDeleteDeviceHWID_success(t *testing.T) {
	client, server := helperSetupClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users/by-username/testUser" {
			if r.Method != "GET" {
				t.Errorf("Ожидали GET метод для GetUUIDByUsername, получили: %s", r.Method)
			}

			response := map[string]interface{}{
				"response": map[string]interface{}{
					"uuid":     "test-uuid",
					"username": "testUser",
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
		}

		if r.URL.Path == "/api/hwid/devices/delete-all" {
			if r.Method != "POST" {
				t.Errorf("Ожидали POST метод, получили: %s", r.Method)
			}

			w.WriteHeader(http.StatusOK)
		}
	})
	defer server.Close()

	err := client.DeleteDeviceHWID(context.Background(), "testUser")
	if err != nil {
		t.Errorf("Удаление устройств должно пройти успешно, ошибка: %v", err)
	}
}
