// Package remnawave содержит определения ошибок, которые могут возникнуть при работе с API remnawave.
package remnawave

import "errors"

// Переменные ошибок, возвращаемые клиентом Remnawave.
var (
	// ErrNotFound возвращается, когда запрашиваемый ресурс не найден.
	ErrNotFound = errors.New("пользователь не найден")
	// ErrInternalServerError возвращается при внутренней ошибке сервера.
	ErrInternalServerError = errors.New("внутренняя ошибка сервера")
	// ErrBadRequestUUID возвращается, если передан некорректный UUID.
	ErrBadRequestUUID = errors.New("неправильный формат uuid")
	// ErrBadRequestCreate возвращается при ошибке валидации данных создания.
	ErrBadRequestCreate = errors.New("неверный формат запроса")
	// ErrBadRequestUsername возвращается, если имя пользователя некорректно.
	ErrBadRequestUsername = errors.New("неверный формат username")
	// ErrLoginFailed возвращается при неудачной попытке входа.
	ErrLoginFailed = errors.New("ошибка входа")
	// ErrReadBody возвращается при ошибке чтения тела ответа.
	ErrReadBody = errors.New("ошибка чтения тела ответа")
	// ErrUnmarshal возвращается при ошибке разбора JSON.
	ErrUnmarshal = errors.New("ошибка разбора JSON")
)
