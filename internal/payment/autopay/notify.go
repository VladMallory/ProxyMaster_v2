package autopay

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type tgNotifier struct {
	token      string
	httpClient *http.Client
}

func newNotifier(token string) *tgNotifier {
	return &tgNotifier{
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (n *tgNotifier) Paid(userID string, months, amountRub int) {
	n.send(
		userID,
		fmt.Sprintf(
			"✅ Автоплатёж прошёл: подписка продлена на %d мес. Списано %d ₽.",
			months,
			amountRub,
		),
	)
}

func (n *tgNotifier) Failed(userID string) {
	n.send(
		userID,
		"⚠️ Автоплатёж не прошёл, поэтому он отключён. "+
			"Оплатите подписку вручную — при оплате карта привяжется заново.",
	)
}

func (n *tgNotifier) send(userID, text string) {
	if n.token == "" {
		return
	}

	chatID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		log.Printf("autopay notify: плохой user id %q: %v", userID, err)

		return
	}

	endpoint := "https://api.telegram.org/bot" + n.token + "/sendMessage"

	form := url.Values{}
	form.Set("chat_id", strconv.FormatInt(chatID, 10))
	form.Set("text", text)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		endpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		log.Printf("autopay notify: %v", err)

		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		log.Printf("autopay notify: %v", err)

		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusBadRequest {
		log.Printf("autopay notify: telegram status %d", resp.StatusCode)
	}
}
