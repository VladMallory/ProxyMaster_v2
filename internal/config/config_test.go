package config

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// func TestParseRemna(t *testing.T) {
// 	t.Parallel()
//
// 	got1, got2 := parseRemna("")
//
// 	if got1 != "https://panel.example.com" {
// 		t.Fatalf("панель не верная")
// 	}
//
// 	if got2 != "5OOdQ2Rb=j9wXGvSp" {
// 		t.Fatalf("да")
// 	}
// }

func Test_parseRemna(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		want  string
		want2 string
	}{
		{
			name:  "ok",
			raw:   "https://panel.example.com/auth/login?5OOdQ2Rb=j9wXGvSp",
			want:  "https://panel.example.com",
			want2: "5OOdQ2Rb=j9wXGvSp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, got2 := parseRemna(tt.raw)
			if got != tt.want {
				t.Errorf("parseRemna() = %v, ожидалось %v", got, tt.want)
			}

			if got != tt.want {
				t.Errorf("parseRemna() = %v, ожидалось %v", got2, tt.want2)
			}
		})
	}
}

func TestParseRemnaInvalidURL(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_PARSE_REMNA") == "1" {
		parseRemna("%")

		return
	}

	cmd := exec.Command(
		os.Args[0],
		"-test.run=^TestParseRemnaInvalidURL$",
	)
	cmd.Env = append(os.Environ(), "TEST_PARSE_REMNA=1")

	err := cmd.Run()

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("ожидался ExitError, получено: %v", err)
	}

	if exitErr.ExitCode() != 1 {
		t.Fatalf("ожидался код выхода 1, получен %d", exitErr.ExitCode())
	}
}

func TestParseRemnaMissingScheme(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_PARSE_REMNA_SCHEME") == "1" {
		parseRemna("//example.com?token=123")

		return
	}

	cmd := exec.Command(
		os.Args[0],
		"-test.run=^TestParseRemnaMissingScheme$",
	)
	cmd.Env = append(os.Environ(), "TEST_PARSE_REMNA_SCHEME=1")

	err := cmd.Run()

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("ожидался ExitError, получено: %v", err)
	}

	if exitErr.ExitCode() != 1 {
		t.Fatalf("ожидался код выхода 1, получен %d", exitErr.ExitCode())
	}
}

func TestParseRemnaMissingQuery(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_PARSE_REMNA_QUERY") == "1" {
		parseRemna("https://panel.example.com")

		return
	}

	cmd := exec.Command(
		os.Args[0],
		"-test.run=^TestParseRemnaMissingQuery$",
	)
	cmd.Env = append(os.Environ(), "TEST_PARSE_REMNA_QUERY=1")

	err := cmd.Run()

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("ожидался ExitError, получено: %v", err)
	}

	if exitErr.ExitCode() != 1 {
		t.Fatalf("ожидался код выхода 1, получен %d", exitErr.ExitCode())
	}
}
