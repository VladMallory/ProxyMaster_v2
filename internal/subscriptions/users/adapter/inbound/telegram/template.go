package telegram

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"text/template"
)

// Читаем файл с диска один раз при инициализации пакета.
// Дальше шаблон живёт в памяти, /start диск не трогает.
var startTemplate = template.Must(loadStartTemplate())

func loadStartTemplate() (*template.Template, error) {
	raw, err := os.ReadFile("assets/start.txt")
	if err != nil {
		return nil, fmt.Errorf("read start template: %w", err)
	}

	return template.New("start").Parse(string(raw))
}

// Данные, которые подставляются в шаблон /start.
type startViewModel struct {
	Name       string
	ExpireDate string
	Device     int
}

func renderStart(vm startViewModel) (string, error) {
	var buf bytes.Buffer
	if err := startTemplate.Execute(&buf, vm); err != nil {
		return "", err
	}

	return buf.String(), nil
}
