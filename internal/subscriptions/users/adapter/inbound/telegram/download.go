package telegram

import "gopkg.in/telebot.v4"

const appStore = "Выберите Ваш регион из App Store. Если не знаете какой у вас, " +
	"кликните сначала на первую кнопку, если ошибка, то на вторую"

const router = "Если ваш роутер на Keenetic OS или OpenWRT, то можно будет поставить " +
	"прям на роутер, но это не прям очень легко, нужно будет чуть-чуть повозиться"

func (h *Handler) handleDownload(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	return c.Edit("Выберите платформу:", h.keys.DownloadApps(), telebot.ModeHTML)
}

func (h *Handler) handleIOS(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	return c.Edit(appStore, h.keys.IOS(), telebot.ModeHTML)
}

func (h *Handler) handleAndroid(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	return c.Edit(
		"Если у вас есть Google Play, то жмите на первую кнопку, если у вас не установлен Google Play, то на вторую:",
		h.keys.Android(),
		telebot.ModeHTML,
	)
}

func (h *Handler) handleLinux(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	return c.Edit("Выберите дистрибитив:", h.keys.Linux(), telebot.ModeHTML)
}

func (h *Handler) handleMacOS(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	return c.Edit("Выберите вариант установки", h.keys.Macos(), telebot.ModeHTML)
}

func (h *Handler) handleRouter(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	return c.Edit(router, h.keys.Router(), telebot.ModeHTML)
}

func (h *Handler) handleBackPlatforms(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	return c.Edit("Выберите платформу:", h.keys.DownloadApps(), telebot.ModeHTML)
}
