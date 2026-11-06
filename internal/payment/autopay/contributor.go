package autopay

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"

	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

// disableCallback уникален для всего бота: по нему platform подписывает
// обработчик нажатия.
const disableCallback = "autopay_off"

const (
	msgAutopayOff = "🔕 Автоплатёж отключён. Больше не будем списывать деньги с карты."
	msgNotBound   = "Автоплатёж не подключён, списывать нечего."
	msgPending    = "⏳ Списание уже в процессе. Отключить автоплатёж можно после того, как оно завершится."
	msgCheckFail  = "⚠️ Не удалось проверить автоплатёж. Попробуйте позже."
	msgDeleteFail = "⚠️ Не удалось отключить автоплатёж. Попробуйте позже."
)

// unbindStore то, что кнопке нужно от хранилища: прочитать запись и удалить её.
type unbindStore interface {
	Get(ctx context.Context, username string) (record, bool, error)
	Delete(ctx context.Context, username string) error
}

// Contributor регистрирует обработчик отключения автоплатежа и отдаёт кнопку
// для экрана тарифов. В главном меню кнопки нет.
type Contributor struct {
	store unbindStore
}

// NewContributor вкладчик меню: кнопка отключения автоплатежа. Собирается в
// main и регистрируется в platform.Registry рядом с остальными кнопками.
// Миграция идемпотентна: кнопка должна работать, даже если роутер не поднялся.
func NewContributor(ctx context.Context, db *sql.DB) (*Contributor, error) {
	st := newStore(db)
	if err := st.Migrate(ctx); err != nil {
		return nil, err
	}

	return newContributor(st), nil
}

func newContributor(st unbindStore) *Contributor {
	return &Contributor{store: st}
}

// Order сразу после кнопки оплаты (у неё 20).
func (c *Contributor) Order() int { return 21 }

// Button кнопка отключения для чужих экранов, например выбора тарифа.
// Второй результат — есть ли у пользователя привязанный автоплатёж:
// пока карта не привязана, показывать нечего.
func (c *Contributor) Button(
	menu *telebot.ReplyMarkup,
	username string,
) (telebot.Btn, bool) {
	_, found, err := c.store.Get(context.Background(), username)
	if err != nil {
		slog.Error(
			"autopay: не удалось проверить привязку",
			"user", username,
			"err", err,
		)

		return telebot.Btn{}, false
	}

	if !found {
		return telebot.Btn{}, false
	}

	return menu.Data("🔕 Отключить автоплатёж", disableCallback), true
}

// Rows пустой: кнопка нужна только на экране тарифов, не в главном меню.
func (c *Contributor) Rows(
	_ *telebot.ReplyMarkup,
	_ platformtg.MenuContent,
) [][]telebot.Btn {
	return nil
}

func (c *Contributor) Handlers() map[string]telebot.HandlerFunc {
	return map[string]telebot.HandlerFunc{
		disableCallback: c.handleDisable,
	}
}

func (c *Contributor) handleDisable(tc telebot.Context) error {
	if err := tc.Respond(); err != nil {
		return err
	}

	username := strconv.FormatInt(tc.Sender().ID, 10)
	ctx := context.Background()

	payment, found, err := c.store.Get(ctx, username)
	if err != nil {
		return tc.Send(msgCheckFail)
	}

	if !found {
		return tc.Edit(msgNotBound, backToMenu())
	}

	// Пока списание висит, удалять запись нельзя: платёж может пройти, а
	// продлевать подписку будет уже некому.
	if payment.PendingPaymentID != "" {
		return tc.Edit(msgPending, backToMenu())
	}

	if err := c.store.Delete(ctx, username); err != nil {
		return tc.Send(msgDeleteFail)
	}

	return tc.Edit(msgAutopayOff, backToMenu())
}

func backToMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}
	btnBack := menu.Data("🏠 В главное меню", "users_back")
	menu.Inline(menu.Row(btnBack))

	return menu
}
