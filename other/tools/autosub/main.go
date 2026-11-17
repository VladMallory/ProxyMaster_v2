// Программа выдаёт ссылки автоподключения Happ и Incy для пользователя
// Remnawave. Username вводится вручную, ссылки печатаются в stdout.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

// config содержит только те настройки, которые нужны этой программе.
// Полный config.Load() не используем: ему нужны telegram и оплата,
// без которых выдача ссылок работать не обязана.
type config struct {
	baseURL    string
	apiKey     string
	token      string
	subDomain  string
	linkSecret string
}

// errUserNotFound отличает отсутствие пользователя от прочих ошибок панели,
// чтобы main мог напечатать понятное сообщение.
var errUserNotFound = errors.New("пользователь не найден")

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fail(err)
	}

	username, err := readUsername(os.Stdin, os.Stdout)
	if err != nil {
		fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := execute(ctx, cfg, username, os.Stdout); err != nil {
		fail(err)
	}
}

// loadConfig читает .env из текущей папки и обязательные переменные панели.
// Пустые AUTO_CONNECT_SUB и LINK_SECRET не считаем ошибкой здесь:
// execute скажет о них отдельно, когда дойдёт до построения ссылок.
func loadConfig() (config, error) {
	_ = godotenv.Load()

	panel := strings.TrimSpace(os.Getenv("REMNA_PANEL"))
	if panel == "" {
		return config{}, errors.New("REMNA_PANEL не задан")
	}

	baseURL, apiKey, err := parsePanel(panel)
	if err != nil {
		return config{}, fmt.Errorf("REMNA_PANEL: %w", err)
	}

	token := strings.TrimSpace(os.Getenv("REMNA_TOKEN"))
	if token == "" {
		return config{}, errors.New("REMNA_TOKEN не задан")
	}

	return config{
		baseURL:    baseURL,
		apiKey:     apiKey,
		token:      token,
		subDomain:  os.Getenv("AUTO_CONNECT_SUB"),
		linkSecret: os.Getenv("LINK_SECRET"),
	}, nil
}

// parsePanel повторяет разбор REMNA_PANEL из internal/config: в ссылке
// панели лежат базовый адрес и ключ API в query-части.
func parsePanel(raw string) (baseURL, apiKey string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}

	if u.Scheme == "" || u.Host == "" {
		return "", "", errors.New("не указаны scheme и host")
	}

	if u.RawQuery == "" {
		return "", "", errors.New("не указан ключ API в query")
	}

	base := url.URL{Scheme: u.Scheme, Host: u.Host}

	return base.String(), u.RawQuery, nil
}

// execute ищет пользователя и печатает обе ссылки автоподключения.
// Ссылки строит тот же autoconnect.Service, что и бот, поэтому результат
// совпадает с кнопками Happ и Incy в меню.
func execute(ctx context.Context, cfg config, username string, out io.Writer) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username не задан")
	}

	links := autoconnect.New(cfg.subDomain, cfg.linkSecret)
	if !links.CheckEnabled() {
		return errors.New("автоподключение не настроено: задайте AUTO_CONNECT_SUB и LINK_SECRET")
	}

	client := platformremnawave.New(cfg.baseURL, cfg.token, zap.NewNop())

	subscription, err := findSubscription(ctx, client, cfg.apiKey, username)
	if err != nil {
		return err
	}

	happ := links.Build(autoconnect.ProviderHapp, subscription)
	incy := links.Build(autoconnect.ProviderIncy, subscription)
	if happ == "" || incy == "" {
		// Build возвращает пустую строку, если ссылку не удалось зашифровать.
		return errors.New("не удалось построить ссылки автоподключения")
	}

	fmt.Fprintf(out, "Happ: %s\nIncy: %s\n", happ, incy)

	return nil
}

// findSubscription запрашивает пользователя по username и возвращает
// его ссылку подписки. Пустую ссылку считаем ошибкой: без неё
// автоподключение построить нельзя.
func findSubscription(
	ctx context.Context,
	client *platformremnawave.Client,
	apiKey, username string,
) (string, error) {
	path := "/api/users/by-username/" + url.PathEscape(username) + "?" + apiKey

	resp, err := platformremnawave.Do[subdomain.APIResponse](
		ctx,
		client,
		http.MethodGet,
		path,
		nil,
	)
	if errors.Is(err, platformremnawave.ErrNotFound) {
		return "", fmt.Errorf("%w: %s", errUserNotFound, username)
	}

	if err != nil {
		return "", fmt.Errorf("запрос пользователя: %w", err)
	}

	subscription := strings.TrimSpace(resp.UserResponse.SubscriptionURL)
	if subscription == "" {
		return "", fmt.Errorf("у пользователя %s нет ссылки подписки", username)
	}

	return subscription, nil
}

// readUsername запрашивает username в терминале. Пустой ввод не отправляем
// в панель: иначе получим непонятную ошибку вместо подсказки.
func readUsername(in io.Reader, out io.Writer) (string, error) {
	fmt.Fprint(out, "Username: ")

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}

		return "", errors.New("username не введён")
	}

	username := strings.TrimSpace(scanner.Text())
	if username == "" {
		return "", errors.New("username не введён")
	}

	return username, nil
}

// fail печатает ошибку в stderr и завершает программу с кодом 1.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "ошибка:", err)
	os.Exit(1)
}
