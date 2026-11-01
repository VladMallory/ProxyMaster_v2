package autoconnect

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
)

// nonceSize размер nonce для AES-GCM. 12 байта, стандарт для этого режима.
const nonceSize = 12

type Service struct {
	baseURL string
	key     []byte
}

// New создаёт сервис автоподключения. Ключ выводится из секрета, чтобы
// LINK_SECRET мог быть любой длины, а не ровно 32 байта.
func New(baseURL, secret string) *Service {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" || secret == "" {
		return &Service{}
	}

	sum := sha256.Sum256([]byte(secret))

	return &Service{
		baseURL: normalizeBaseURL(baseURL),
		key:     sum[:],
	}
}

// CheckEnabled сообщает, настроено ли автоподключение.
func (s *Service) CheckEnabled() bool {
	return s != nil && s.baseURL != "" && len(s.key) > 0
}

// Build возвращает URL страницы приложения с зашифрованным токеном подписки.
// Адрес подписки в ссылке не виден: его можно получить только на сервере.
func (s *Service) Build(provider, subscriptionURL string) string {
	if !s.CheckEnabled() || subscriptionURL == "" {
		return ""
	}

	token, err := s.seal(subscriptionURL)
	if err != nil {
		return ""
	}

	query := url.Values{}
	query.Set("t", token)

	return s.baseURL + "/" + provider + "?" + query.Encode()
}

// Open расшифровывает токен и возвращает ссылку подписки.
// false означает, что токен подделан или собран другим секретом.
func (s *Service) Open(token string) (string, bool) {
	if !s.CheckEnabled() || token == "" {
		return "", false
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) <= nonceSize {
		return "", false
	}

	gcm, err := s.aead()
	if err != nil {
		return "", false
	}

	plain, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", false
	}

	return string(plain), true
}

// seal шифрует ссылку подписки и приклеивает nonce в начало результата.
func (s *Service) seal(subscriptionURL string) (string, error) {
	gcm, err := s.aead()
	if err != nil {
		return "", err
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	// Первым аргументом передаём уже заполненный nonce, поэтому шифротекст
	// дописывается сразу после него.
	sealed := gcm.Seal(nonce, nonce, []byte(subscriptionURL), nil)

	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Service) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	return strings.TrimRight(raw, "/")
}
