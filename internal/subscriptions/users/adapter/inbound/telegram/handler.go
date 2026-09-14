package telegram

import (
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/inbound/telegram/keyboard"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
)

type Handler struct {
	useCase   userscase.UserUseCase
	trialDays int
	keys      *keyboard.Keyboard
}

func NewHandler(
	useCase userscase.UserUseCase,
	supportURL string,
	trialDays int,
) *Handler {
	return &Handler{
		useCase:   useCase,
		trialDays: trialDays,
		keys:      keyboard.New(supportURL),
	}
}
