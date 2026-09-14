package telegram

import (
	"bytes"
	"os"
	"text/template"
)

// startTemplate грузится один раз при старте пакета.
// Дальше /start диск не трогает.
var startTemplate = template.Must(loadStartTemplate())

func loadStartTemplate() (*template.Template, error) {
	paths := []string{"assets/start.txt", "../../assets/start.txt", "../../../assets/start.txt"}
	var lastErr error

	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err == nil {
			return template.New("start").Parse(string(raw))
		}

		lastErr = err
	}

	return nil, lastErr
}

// startViewModel данные для шаблона /start.
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
