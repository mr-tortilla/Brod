package node

import (
	"encoding/base64"
	"testing"
)

const clashConfig = `
mixed-port: 7890
proxies:
  - name: "🇳🇱 NL reality"
    type: vless
    server: nl.example
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
    network: tcp
    tls: true
    flow: xtls-rprx-vision
    servername: www.example.com
    client-fingerprint: chrome
    reality-opts:
      public-key: PUBKEY
      short-id: ab12
  - name: DE ws
    type: vless
    server: 192.0.2.4
    port: "8443"
    uuid: 11111111-2222-3333-4444-555555555555
    network: ws
    tls: true
    servername: de.example
    ws-opts:
      path: /ws
      headers:
        Host: cdn.example
  - name: FI grpc
    type: vless
    server: fi.example
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
    network: grpc
    tls: true
    grpc-opts:
      grpc-service-name: svc
  - name: trojan node
    type: trojan
    server: t.example
    port: 443
    password: FAKE_TEST_PASSWORD
  - name: hy2-test
    type: hysteria2
    server: hy2.example
    port: 443
    password: fake-password
    up: 50 Mbps
    down: 100 Mbps
    sni: hy2.example
    obfs: salamander
    obfs-password: fake-obfs
    skip-cert-verify: true
  - name: tuic-test
    type: tuic
    server: tuic.example
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
    password: fake-password
    congestion-controller: bbr
    udp-relay-mode: native
    alpn: [h3]
proxy-groups:
  - name: auto
    type: url-test
    proxies: ["🇳🇱 NL reality"]
`

func TestParseClash(t *testing.T) {
	nodes, skipped, err := ParseText(clashConfig)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 5 || skipped != 1 {
		t.Fatalf("nodes=%d skipped=%d", len(nodes), skipped)
	}
	r, ws, g := nodes[0], nodes[1], nodes[2]
	if r.Security != "reality" || r.PublicKey != "PUBKEY" || r.ShortID != "ab12" || r.SNI != "www.example.com" ||
		r.Flow != "xtls-rprx-vision" || r.Fingerprint != "chrome" || r.Port != 443 {
		t.Fatalf("reality: %+v", r)
	}
	if ws.Network != "ws" || ws.Path != "/ws" || ws.Host != "cdn.example" || ws.Port != 8443 || ws.Security != "tls" {
		t.Fatalf("ws: %+v", ws)
	}
	if g.Network != "grpc" || g.ServiceName != "svc" {
		t.Fatalf("grpc: %+v", g)
	}
	hy2, tuic := nodes[3], nodes[4]
	if hy2.Protocol != "hysteria2" || hy2.UpMbps != 50 || hy2.DownMbps != 100 || hy2.ObfsType != "salamander" || !hy2.Insecure {
		t.Fatalf("hysteria2: %+v", hy2)
	}
	if tuic.Protocol != "tuic" || tuic.CongestionControl != "bbr" || tuic.UDPRelayMode != "native" || tuic.Password != "fake-password" {
		t.Fatalf("tuic: %+v", tuic)
	}

	// та же подписка в base64
	nodes, _, err = ParseText(base64.StdEncoding.EncodeToString([]byte(clashConfig)))
	if err != nil || len(nodes) != 5 {
		t.Fatalf("base64 YAML: %v, %d", err, len(nodes))
	}
}

// Сервер из YAML и тот же сервер из ссылки — одно подключение с одинаковым ID.
func TestClashSameIDAsLink(t *testing.T) {
	nodes, _, err := ParseClash(clashConfig)
	if err != nil {
		t.Fatal(err)
	}
	link, err := ParseLink(Link(nodes[0]))
	if err != nil || link.ID != nodes[0].ID {
		t.Fatalf("ID из YAML %s, из ссылки %s (%v)", nodes[0].ID, link.ID, err)
	}
}
