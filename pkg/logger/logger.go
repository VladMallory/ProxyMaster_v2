// Package logger предоставляет абстракцию для структурированного логирования.
// Он скрывает реализацию (zap) за интерфейсом
package logger

// Field описывает поле лога (ключ-значение).
// zap требует писать ключ-значение.
type Field struct {
	Key   string
	Value any
}

// Logger интерфейс для логирования.
// Позволяет внедрять зависимости и менять реализацию логгера.
// Если захотим сменить логгер на другой, то нужно будет
// переписать этот один файл
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
	Named(name string) Logger
	Sync() error
}
