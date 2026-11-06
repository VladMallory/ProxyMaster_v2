package main

import (
	"testing"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
)

func TestNewWebServerDisabledWithoutConfig(t *testing.T) {
	t.Parallel()

	if got := newWebServer(":8080", autoconnect.New("", "secret"), "Shadowfade"); got != nil {
		t.Fatalf("newWebServer = %+v, want nil", got)
	}

	svc := autoconnect.New("connect.example.com", "secret")

	if got := newWebServer("", svc, "Shadowfade"); got != nil {
		t.Fatalf("newWebServer = %+v, want nil", got)
	}
}

func TestNewWebServerUsesAddress(t *testing.T) {
	t.Parallel()

	got := newWebServer(":9090", autoconnect.New("connect.example.com", "secret"), "Shadowfade")
	if got == nil {
		t.Fatal("newWebServer = nil, want server")
	}

	if got.Addr != ":9090" {
		t.Fatalf("Addr = %q, want :9090", got.Addr)
	}
}
