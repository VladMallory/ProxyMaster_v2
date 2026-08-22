package telegram

import (
	"fmt"
	"time"
)

var monthsRu = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

func formatExpireDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d",
		t.Day(), monthsRu[t.Month()-1], t.Year(),
	)
}
