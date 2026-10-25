package telegram

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"
)

// startTemplate грузится один раз при старте пакета.
// Дальше /start диск не трогает.
var startTemplate = template.Must(loadStartTemplate())

func loadStartTemplate() (*template.Template, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var lastErr error

	// Ищем assets вверх по дереву: при запуске из корня и при go test
	// из каталога пакета файл находится на разной глубине.
	for {
		raw, rerr := os.ReadFile(filepath.Join(dir, "assets", "start.txt"))
		if rerr == nil {
			return template.New("start").Parse(string(raw))
		}

		lastErr = rerr

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, lastErr
		}

		dir = parent
	}
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
