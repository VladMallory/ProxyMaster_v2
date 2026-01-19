package remnawave

import "errors"

// Переменные ошибок, возвращаемые клиентом Remnawave.
var (
	// ErrNotFound возвращается, когда запрашиваемый ресурс не найден.
	ErrNotFound = errors.New("пользователь не найден")
	// ErrUndefined возвращается, когда ошибка не определена.
	ErrUndefined = errors.New("неопределенная ошибка")
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
	// ErrBadStatusCode возвращается при ошибке статуса ответа.
	ErrBadStatusCode = errors.New("bad status code error")
	// ErrFailedToReadBody возвращается при ошибке чтения тела ответа.
	ErrFailedToReadBody = errors.New("read response body error")
	// ErrFailedToUnmarshal возвращается при ошибке разбора JSON.
	ErrFailedToUnmarshal = errors.New("failed to unmarshal json")
	// ErrEmptyURL возвращается при пустой ссылке.
	ErrEmptyURL = errors.New("empty url")
	// ErrFailedToMakeRequest возвращается при ошибке создания запроса.
	ErrFailedToMakeRequest = errors.New("failed to make request")
	// ErrFailedToMakeResponse возвращается при ошибке преобразования ответа.
	ErrFailedToMakeResponse = errors.New("failed to make response")
	// ErrFailedToMarshal возвращается при ошибке сериализации JSON.
	ErrFailedToMarshal = errors.New("failed to marshal json")
	// ErrFailedToDoRequest возвращается при ошибке выполнения запроса.
	ErrFailedToDoRequest = errors.New("failed to do request")
	// ErrFailedToIncreaseSubscriptionPeriod возвращается при ошибке увеличения периода подписки.
	ErrFailedToIncreaseSubscriptionPeriod = errors.New("failed to increase subscription period")
	// ErrUUIDorUsernameIsNill
	ErrUUIDorUsernameIsNill = errors.New("uuid or username is nil")
	// ErrDevicesNotSet
	ErrDevicesNotSet = errors.New("не указано кол-во устройств в методе SetDevices")
	// ErrBadRequest
	ErrBadRequest = errors.New("bad request")
	// ErrDaysNotNill
	ErrDaysNotNill = errors.New("дней не может быть ноль")
)
