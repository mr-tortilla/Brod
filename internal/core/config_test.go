package core

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	sjson "github.com/sagernet/sing/common/json"

	"github.com/mr-tortilla/brod/internal/node"
)

// Проверяет, что sing-box принимает собранный конфиг. Start не вызывается,
// чтобы тест не трогал системный прокси и сетевые интерфейсы.
func TestConfigAccepted(t *testing.T) {
	nodes := []node.Node{
		{Server: "example.com", Port: 443, UUID: "11111111-2222-3333-4444-555555555555", Flow: "xtls-rprx-vision",
			Network: "tcp", Security: "reality", SNI: "www.google.com", PublicKey: "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0", ShortID: "ab"},
		{Server: "1.2.3.4", Port: 8443, UUID: "11111111-2222-3333-4444-555555555555", Network: "ws", Security: "tls", Path: "/ws", Host: "h.example", Fingerprint: "firefox"},
		{Server: "1.2.3.4", Port: 8443, UUID: "11111111-2222-3333-4444-555555555555", Network: "grpc", Security: "tls", ServiceName: "svc"},
	}
	bypass, _ := ParseBypass([]string{"ru", "рф", "10.0.0.0/8"})
	for _, mode := range []Mode{ModeProxy, ModeTUN} {
		for _, n := range nodes {
			opts := Options{
				Mode: mode, Bypass: bypass, DNS: DNS{Direct: "77.88.8.8", Remote: "1.1.1.1"},
				BypassLAN: true, DNSStrategy: "ipv4_only", TunStack: "mixed", TunMTU: 9000, StrictRoute: true,
			}
			cfg, err := buildConfig(n, opts, 20808, filepath.Join(t.TempDir(), "cache.db"))
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(cfg)
			ctx, cancel := context.WithCancel(include.Context(context.Background()))
			options, err := sjson.UnmarshalExtendedContext[option.Options](ctx, data)
			if err != nil {
				t.Fatalf("%s/%s: %v", mode, n.Network, err)
			}
			instance, err := box.New(box.Options{Context: ctx, Options: options})
			if err != nil {
				t.Fatalf("%s/%s: %v", mode, n.Network, err)
			}
			_ = instance.Close()
			cancel()
		}
	}
}

func TestUnsupportedTransport(t *testing.T) {
	if _, err := buildConfig(node.Node{Network: "xhttp", Security: "none"}, Options{Mode: ModeProxy}, 1, ""); err == nil {
		t.Fatal("ожидалась ошибка для xhttp")
	}
}

func TestParseBypass(t *testing.T) {
	b, invalid := ParseBypass([]string{
		".ru", "*.su", "рф", "https://Сайт.рф/путь?q=1", "vk.com:443", "ya.ru", "ru",
		"1.2.3.4", "10.0.0.1/8", "2001:db8::1", "# комментарий", "", "не домен!",
	})
	wantDomains := []string{"ru", "su", "xn--p1ai", "xn--80aswg.xn--p1ai", "vk.com", "ya.ru"}
	wantCIDRs := []string{"1.2.3.4/32", "10.0.0.0/8", "2001:db8::1/128"}
	if !slices.Equal(b.Domains, wantDomains) {
		t.Errorf("domains = %v, want %v", b.Domains, wantDomains)
	}
	if !slices.Equal(b.CIDRs, wantCIDRs) {
		t.Errorf("cidrs = %v, want %v", b.CIDRs, wantCIDRs)
	}
	if len(invalid) != 1 || invalid[0] != "не домен!" {
		t.Errorf("invalid = %v", invalid)
	}
}
