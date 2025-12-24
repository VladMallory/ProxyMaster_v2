package remnawave

import "errors"

var (
	ErrNotFound            = errors.New("пользователь не найден")
	ErrInternalServerError = errors.New("внутренняя ошибка сервера")
	ErrBadRequestUUID      = errors.New("неправильный формат uuid")
	ErrBadRequestCreate    = errors.New("неверный формат запроса")
	ErrBadRequestUsername  = errors.New("неверный формат username")
)
